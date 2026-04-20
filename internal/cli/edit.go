package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
	"github.com/danielgtaylor/shorthand/v2"
	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func (c *CLI) addEditCommand(root *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "edit <uri> [patch ...]",
		Short: "Fetch a resource, edit it locally, then send it back",
		Args:  cobra.MinimumNArgs(1),
		RunE:  c.runEdit,
	}
	cmd.Flags().StringP("edit-format", "e", "json", "Editor file format: json or yaml")
	cmd.Flags().Bool("dry-run", false, "Show the diff without sending the update")
	cmd.Flags().BoolP("rsh-yes", "y", false, "Skip the confirmation prompt")
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
	prepared, err := c.prepareRequest(rawURL, profileName, opts, nil, nil, false)
	if err != nil {
		return err
	}
	defer c.closePreparedTransport(prepared)
	rawURL = prepared.rawURL

	httpResp, err := c.sendPreparedRequest(requestContext(cmd), "GET", prepared)
	if err != nil {
		return fmt.Errorf("network error for GET %s: %w", rawURL, err)
	}

	if verbose, _ := cmd.Flags().GetCount("rsh-verbose"); verbose >= 1 && httpResp.Request != nil {
		c.logVerbose(httpResp)
	}

	resp, err := c.normalizeHTTPResponse(httpResp, maxBodyBytes(cmd))
	if err != nil {
		return err
	}
	if code := output.StatusToExitCode(resp.Status); code != 0 {
		if formatErr := c.formatResponse(cmd, resp); formatErr != nil {
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

	originalText, err := marshalEditValue(editFormat, resp.Body)
	if err != nil {
		return fmt.Errorf("edit: marshal resource: %w", err)
	}

	tmp, err := os.CreateTemp("", "restish-edit-*"+ext)
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
	} else {
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

	if bytes.Equal(originalText, editedText) {
		fmt.Fprintln(c.Stderr, "No changes made.")
		return nil
	}

	diff := unifiedDiff("original", "modified", string(originalText), string(editedText))
	if diff != "" {
		if output.ColorEnabled(c.Stdout) {
			if lexer := lexers.Get("diff"); lexer != nil {
				if colored, err := output.HighlightWithLexer(lexer, []byte(diff)); err == nil {
					diff = string(colored)
				}
			}
		}
		fmt.Fprint(c.Stdout, diff)
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if dryRun {
		return nil
	}

	skipPrompt, _ := cmd.Flags().GetBool("rsh-yes")
	if !skipPrompt {
		ok, err := c.confirmEdit()
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
	updateContentType := mimeType
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

	updateOpts := opts
	updateOpts.Headers = append([]string{}, opts.Headers...)
	updateOpts.Headers = append(updateOpts.Headers, "Content-Type: "+actualContentType)
	if etag := resp.Headers["Etag"]; etag != "" {
		updateOpts.Headers = append(updateOpts.Headers, "If-Match: "+etag)
	} else if lastModified := resp.Headers["Last-Modified"]; lastModified != "" {
		updateOpts.Headers = append(updateOpts.Headers, "If-Unmodified-Since: "+lastModified)
	}

	updateResp, err := request.Do(requestContext(cmd), updateMethod, rawURL, bytes.NewReader(encoded), updateOpts)
	if err != nil {
		return fmt.Errorf("network error for %s %s: %w", updateMethod, rawURL, err)
	}
	if verbose, _ := cmd.Flags().GetCount("rsh-verbose"); verbose >= 1 && updateResp.Request != nil {
		c.logVerbose(updateResp)
	}

	normalized, err := output.Normalize(updateResp, c.content, maxBodyBytes(cmd))
	if err != nil {
		return err
	}
	if err := c.formatResponse(cmd, normalized); err != nil {
		return err
	}

	ignoreStatus, _ := cmd.Flags().GetBool("rsh-ignore-status-code")
	if !ignoreStatus {
		if code := output.StatusToExitCode(normalized.Status); code != 0 {
			return &ExitCodeError{Code: code}
		}
	}
	return nil
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
	var parts []string
	var cur strings.Builder
	var quote rune
	escaped := false
	argStarted := false

	flush := func() {
		if argStarted || cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
			argStarted = false
		}
	}

	for _, r := range s {
		switch {
		case escaped:
			argStarted = true
			cur.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			argStarted = true
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				argStarted = true
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			argStarted = true
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			argStarted = true
			cur.WriteRune(r)
		}
	}
	if escaped {
		argStarted = true
		cur.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return parts, nil
}

func (c *CLI) confirmEdit() (bool, error) {
	fmt.Fprint(c.Stderr, "Continue? [Y/n] ")
	reader := bufio.NewReader(c.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("edit: read confirmation: %w", err)
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "" || answer == "y" || answer == "yes", nil
}

func supportsMergePatch(headers map[string]string) bool {
	if value := headers["Accept-Patch"]; strings.Contains(strings.ToLower(value), "application/merge-patch+json") {
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
