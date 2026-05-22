package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/danielgtaylor/shorthand/v2"
	"github.com/google/shlex"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func (c *CLI) addEditCommand(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:     "edit <uri> [patch ...]",
		Short:   "Fetch a resource, edit it locally, then send it back",
		Long:    editLong,
		GroupID: rootGroupUtility,
		Example: fmt.Sprintf(`  %s edit https://api.example.com/items/123
  %s edit https://api.example.com/items/123 'name: Ada' --dry-run
  %s edit demo/items/123 --no-editor`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args: usageMinimumNArgs(1),
		RunE: c.runEdit,
	}
	cmd.Flags().StringP("edit-format", "e", "json", "Editor file format: json or yaml")
	cmd.Flags().Bool("no-editor", false, "Do not open an editor; with no patch args, print the editable resource")
	cmd.Flags().Bool("dry-run", false, "Show the diff without sending the update")
	cmd.Flags().BoolP("yes", "y", false, "Skip the confirmation prompt")
	root.AddCommand(cmd)
}

func (c *CLI) runEdit(cmd *cobra.Command, args []string) error {
	rawURL := args[0]
	patchArgs := args[1:]
	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	profileName := c.profileFromCmd(cmd)
	opts.ContentType = ""
	prepared, err := c.prepareRequest(requestContext(cmd), "GET", rawURL, profileName, opts, nil, nil, false, authHandlerOptions{}, nil, false, "")
	if err != nil {
		return err
	}
	defer c.closePreparedTransport(prepared)
	rawURL = prepared.rawURL

	httpResp, err := c.sendPreparedRequest(requestContext(cmd), "GET", prepared)
	if err != nil {
		return fmt.Errorf("network error for GET %s: %w", rawURL, err)
	}

	resp, err := c.normalizeHTTPResponse(httpResp, maxBodyBytes(cmd))
	if err != nil {
		return err
	}
	if v := globalFlagsFromContext(requestContext(cmd)).Verbose; v >= 1 {
		c.logVerboseResponseBody(resp)
	}
	if code := output.StatusToExitCode(resp.Status); code != 0 {
		if formatErr := c.formatResponse(cmd, resp, nil); formatErr != nil {
			return formatErr
		}
		return &ExitCodeError{Code: code}
	}
	if resp.Body == nil {
		return fmt.Errorf("edit: resource returned an empty body")
	}

	editFormat, _ := cmd.Flags().GetString("edit-format")
	editFormat = strings.ToLower(editFormat)
	mimeType, ext, err := editFormatInfo(editFormat)
	if err != nil {
		return err
	}
	wireContentType := editWireContentType(resp.Headers)

	originalText, err := marshalEditValue(editFormat, resp.Body)
	if err != nil {
		return fmt.Errorf("edit: marshal resource: %w", err)
	}

	editTmpDir := filepath.Join(c.cacheDir(), "edit")
	if err := os.MkdirAll(editTmpDir, 0o700); err != nil {
		return fmt.Errorf("edit: create temp dir: %w", err)
	}
	tmp, err := os.CreateTemp(editTmpDir, "restish-edit-*"+ext)
	if err != nil {
		return fmt.Errorf("edit: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(originalText); err != nil {
		tmp.Close()
		return fmt.Errorf("edit: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("edit: close temp file: %w", err)
	}

	noEditor, _ := cmd.Flags().GetBool("no-editor")
	if noEditor && len(patchArgs) == 0 {
		if _, err := c.Stdout.Write(originalText); err != nil {
			return fmt.Errorf("edit: write editable resource: %w", err)
		}
		return c.flushStdout()
	}

	editedValue := resp.Body
	editedText := originalText
	if len(patchArgs) > 0 {
		editedValue, err = shorthand.Unmarshal(strings.Join(patchArgs, " "), shorthand.ParseOptions{
			EnableFileInput:       true,
			EnableObjectDetection: true,
		}, resp.Body)
		if err != nil {
			return fmt.Errorf("edit: apply patch args: %w", err)
		}
		editedText, err = marshalEditValue(editFormat, editedValue)
		if err != nil {
			return fmt.Errorf("edit: marshal patched value: %w", err)
		}
	}

	if len(patchArgs) == 0 {
		editorCmd, err := c.editorCommand(tmpPath)
		if err != nil {
			return err
		}
		editorCmd.Stdin = c.Stdin
		editorCmd.Stdout = c.Stdout
		editorCmd.Stderr = c.Stderr
		if err := editorCmd.Run(); err != nil {
			return fmt.Errorf("edit: editor: %w", err)
		}

		editedText, err = os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("edit: read edited file: %w", err)
		}

		editedValue, err = c.content.Decode(mimeType, editedText)
		if err != nil {
			return fmt.Errorf("edit: parse edited file: %w", err)
		}
	}

	normalizedEditedText, err := marshalEditValue(editFormat, editedValue)
	if err != nil {
		return fmt.Errorf("edit: marshal edited value: %w", err)
	}
	editedText = normalizedEditedText

	if bytes.Equal(originalText, editedText) {
		fmt.Fprintln(c.Stderr, "No changes made.")
		return nil
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	diff := unifiedDiff("original", "modified", string(originalText), string(editedText))
	if diff != "" {
		diffOut := c.editDiffOutput(cmd, dryRun)
		if output.ColorEnabled(diffOut) {
			if lexer := lexers.Get("diff"); lexer != nil {
				if colored, err := output.HighlightWithLexer(lexer, []byte(diff)); err == nil {
					diff = string(colored)
				}
			}
		}
		fmt.Fprint(diffOut, diff)
	}

	if dryRun {
		return nil
	}

	hasPrecondition := output.Header(resp.Headers, "Etag") != "" || output.Header(resp.Headers, "Last-Modified") != ""
	if !hasPrecondition {
		c.warnf("edit: response did not include ETag or Last-Modified; update is not guarded against concurrent edits")
	}

	skipPrompt, _ := cmd.Flags().GetBool("yes")
	if !skipPrompt {
		ok, err := c.Confirm(cmd.Context(), "Continue? [Y/n] ")
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(c.Stderr, "Aborted.")
			return nil
		}
	}

	updateMethod := "PUT"
	updateBody := any(editedValue)
	updateContentType := wireContentType
	if supportsMergePatch(resp.Headers) {
		updateMethod = "PATCH"
		updateBody = buildMergePatch(resp.Body, editedValue)
		updateContentType = "application/merge-patch+json"
	}

	var encoded []byte
	actualContentType := updateContentType
	if updateContentType == "application/merge-patch+json" {
		encoded, err = json.Marshal(updateBody)
	} else {
		encoded, actualContentType, err = c.content.EncodeWithType(updateContentType, updateBody)
	}
	if err != nil {
		return fmt.Errorf("edit: encode update body: %w", err)
	}

	updateOpts := prepared.opts
	updateOpts.Headers = append([]string{}, prepared.opts.Headers...)
	updateOpts.Headers = append(updateOpts.Headers, "Content-Type: "+actualContentType)
	if etag := output.Header(resp.Headers, "Etag"); etag != "" {
		updateOpts.Headers = append(updateOpts.Headers, "If-Match: "+etag)
	} else if lastModified := output.Header(resp.Headers, "Last-Modified"); lastModified != "" {
		updateOpts.Headers = append(updateOpts.Headers, "If-Unmodified-Since: "+lastModified)
	}

	updatePrepared := *prepared
	updatePrepared.bodyRaw = encoded
	updatePrepared.opts = updateOpts
	updateResp, err := c.sendPreparedRequest(requestContext(cmd), updateMethod, &updatePrepared)
	if err != nil {
		return fmt.Errorf("network error for %s %s: %w", updateMethod, rawURL, err)
	}
	normalized, err := output.Normalize(updateResp, c.content, maxBodyBytes(cmd))
	if err != nil {
		return err
	}
	if v := globalFlagsFromContext(requestContext(cmd)).Verbose; v >= 1 {
		c.logVerboseResponseBody(normalized)
	}
	if err := c.formatResponse(cmd, normalized, nil); err != nil {
		return err
	}

	ignoreStatus := globalFlagsFromContext(requestContext(cmd)).IgnoreStatus
	if !ignoreStatus {
		if code := output.StatusToExitCode(normalized.Status); code != 0 {
			return &ExitCodeError{Code: code}
		}
	}
	return nil
}

func (c *CLI) editDiffOutput(cmd *cobra.Command, dryRun bool) io.Writer {
	if dryRun {
		return c.Stdout
	}
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.OutputFormatSet || gf.PrintSet || explicitOutputFilter(gf) {
		return c.Stderr
	}
	return c.Stdout
}

func editFormatInfo(name string) (mimeType, ext string, err error) {
	switch name {
	case "json":
		return "application/json", ".json", nil
	case "yaml":
		return "application/yaml", ".yaml", nil
	default:
		return "", "", fmt.Errorf("unsupported edit format %q; expected json or yaml", name)
	}
}

func editWireContentType(headers map[string][]string) string {
	contentType := strings.TrimSpace(strings.Split(output.Header(headers, "Content-Type"), ";")[0])
	if contentType == "" {
		return "application/json"
	}
	return contentType
}

func marshalEditValue(format string, v any) ([]byte, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(data, '\n'), nil
	case "yaml":
		return yaml.Marshal(v)
	default:
		return nil, fmt.Errorf("unsupported edit format %q", format)
	}
}

func (c *CLI) editorCommand(path string) (*exec.Cmd, error) {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		return nil, fmt.Errorf("no editor found; set $VISUAL or $EDITOR")
	}
	parts, err := splitCommandLine(editor)
	if err != nil {
		return nil, fmt.Errorf("parse editor command: %w", err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("no editor found; set $VISUAL or $EDITOR")
	}
	return exec.Command(parts[0], append(parts[1:], path)...), nil
}

func splitCommandLine(s string) ([]string, error) {
	return shlex.Split(s)
}

func supportsMergePatch(headers map[string][]string) bool {
	if value := output.Header(headers, "Accept-Patch"); strings.Contains(strings.ToLower(value), "application/merge-patch+json") {
		return true
	}
	return false
}

func buildMergePatch(original, modified any) any {
	if reflect.DeepEqual(original, modified) {
		return map[string]any{}
	}

	origMap, okOrig := original.(map[string]any)
	modMap, okMod := modified.(map[string]any)
	if !okOrig || !okMod {
		return modified
	}

	patch := map[string]any{}
	seen := map[string]struct{}{}
	for key, modValue := range modMap {
		seen[key] = struct{}{}
		origValue, ok := origMap[key]
		if !ok {
			patch[key] = modValue
			continue
		}
		if reflect.DeepEqual(origValue, modValue) {
			continue
		}
		if origChild, ok1 := origValue.(map[string]any); ok1 {
			if modChild, ok2 := modValue.(map[string]any); ok2 {
				childPatch := buildMergePatch(origChild, modChild)
				if childMap, ok := childPatch.(map[string]any); ok && len(childMap) == 0 {
					continue
				}
				patch[key] = childPatch
				continue
			}
		}
		patch[key] = modValue
	}
	for key := range origMap {
		if _, ok := seen[key]; !ok {
			patch[key] = nil
		}
	}
	return patch
}

func unifiedDiff(oldName, newName, oldText, newText string) string {
	edits := myers.ComputeEdits(span.URIFromPath(oldName), oldText, newText)
	if len(edits) == 0 {
		return ""
	}
	return fmt.Sprint(gotextdiff.ToUnified(oldName, newName, oldText, edits))
}
