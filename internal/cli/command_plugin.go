package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
	"github.com/danielgtaylor/restish/v2/internal/spec"
	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
	"github.com/spf13/cobra"
)

var commandPluginDecMode = func() cbor.DecMode {
	dm, err := cbor.DecOptions{
		DefaultMapType: reflect.TypeOf(map[string]any{}),
	}.DecMode()
	if err != nil {
		panic("command plugin: creating CBOR decode mode: " + err.Error())
	}
	return dm
}()

type CommandDecl struct {
	Name             string `cbor:"name" json:"name"`
	Short            string `cbor:"short" json:"short"`
	Long             string `cbor:"long" json:"long"`
	PassthroughStdio bool   `cbor:"passthrough_stdio" json:"passthrough_stdio"`
}

type CommandsResponse struct {
	Commands []CommandDecl `cbor:"commands" json:"commands"`
}

func loadCommandPluginCommands(path string) ([]CommandDecl, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--rsh-plugin-commands")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil, nil
	}

	var resp CommandsResponse
	if err := commandPluginDecMode.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("plugin %s: commands decode: %w", filepath.Base(path), err)
	}
	return resp.Commands, nil
}

func (c *CLI) addCommandPlugins(root *cobra.Command) {
	for _, p := range c.pluginsForHook("command") {
		cmds, err := loadCommandPluginCommands(p.Path)
		if err != nil || len(cmds) == 0 {
			continue
		}
		for _, decl := range cmds {
			decl := decl
			pluginPath := p.Path
			root.AddCommand(&cobra.Command{
				Use:   decl.Name,
				Short: decl.Short,
				Long:  decl.Long,
				Args:  cobra.ArbitraryArgs,
				RunE: func(cmd *cobra.Command, args []string) error {
					return c.runCommandPlugin(cmd, pluginPath, decl, args)
				},
			})
		}
	}
}

type commandPluginWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *commandPluginWriter) WriteMessage(v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return pluginwire.WriteMessage(w.w, v)
}

func (c *CLI) runCommandPlugin(cmd *cobra.Command, pluginPath string, decl CommandDecl, args []string) error {
	proc := exec.Command(pluginPath, append(terminalContextFlags(c), args...)...)
	proc.Stderr = cmd.ErrOrStderr()

	stdinPipe, err := proc.StdinPipe()
	if err != nil {
		return fmt.Errorf("command plugin: stdin pipe: %w", err)
	}
	stdoutPipe, err := proc.StdoutPipe()
	if err != nil {
		return fmt.Errorf("command plugin: stdout pipe: %w", err)
	}
	if err := proc.Start(); err != nil {
		return fmt.Errorf("command plugin: start: %w", err)
	}

	writer := &commandPluginWriter{w: stdinPipe}
	initMsg := map[string]any{
		"type":    "init",
		"command": decl.Name,
		"args":    args,
	}
	if err := writer.WriteMessage(initMsg); err != nil {
		_ = proc.Process.Kill()
		return fmt.Errorf("command plugin: send init: %w", err)
	}

	if decl.PassthroughStdio {
		go c.streamPluginStdin(writer)
	}

	var loopErr error
	for {
		var msg map[string]any
		if err := pluginwire.ReadMessage(stdoutPipe, &msg); err != nil {
			if isEOFLike(err) {
				loopErr = fmt.Errorf("command plugin %s: process died unexpectedly", filepath.Base(pluginPath))
			} else {
				loopErr = fmt.Errorf("command plugin %s: read message: %w", filepath.Base(pluginPath), err)
			}
			break
		}

		done, err := c.handleCommandPluginMessage(cmd, writer, msg)
		if err != nil {
			loopErr = err
			break
		}
		if done {
			break
		}
	}

	_ = stdinPipe.Close()
	_ = proc.Wait()
	return loopErr
}

func (c *CLI) streamPluginStdin(writer *commandPluginWriter) {
	buf := make([]byte, 32*1024)
	for {
		n, err := c.Stdin.Read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			if writeErr := writer.WriteMessage(map[string]any{
				"type": "stdin-data",
				"data": data,
			}); writeErr != nil {
				return
			}
		}
		if err == io.EOF {
			_ = writer.WriteMessage(map[string]any{"type": "stdin-close"})
			return
		}
		if err != nil {
			return
		}
	}
}

func (c *CLI) handleCommandPluginMessage(cmd *cobra.Command, writer *commandPluginWriter, msg map[string]any) (bool, error) {
	msgType, _ := msg["type"].(string)
	switch msgType {
	case "done":
		code := msgInt(msg, "exit_code")
		if code != 0 {
			return true, &ExitCodeError{Code: code}
		}
		return true, nil
	case "http-request":
		return false, c.handlePluginHTTPRequest(cmd, writer, msg)
	case "api-spec":
		return false, c.handlePluginAPISpec(writer, msg)
	case "response":
		return false, c.handlePluginResponse(cmd, msg)
	case "stdout-data":
		if data := msgBytes(msg["data"]); len(data) > 0 {
			_, _ = c.Stdout.Write(data)
		}
	case "stderr-data":
		if data := msgBytes(msg["data"]); len(data) > 0 {
			_, _ = cmd.ErrOrStderr().Write(data)
		}
	case "progress", "spinner", "log":
		if text, _ := msg["text"].(string); text != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), text)
		}
	case "warn":
		if text, _ := msg["text"].(string); text != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", text)
		}
	}
	return false, nil
}

func (c *CLI) handlePluginHTTPRequest(cmd *cobra.Command, writer *commandPluginWriter, msg map[string]any) error {
	method, _ := msg["method"].(string)
	if method == "" {
		method = "GET"
	}
	rawURL, _ := msg["uri"].(string)

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	profileName, _ := cmd.Flags().GetString("rsh-profile")
	if profileName == "" {
		profileName = "default"
	}
	rawURL, _, opts = c.applyAPIProfile(rawURL, profileName, opts)
	origOnReq := opts.OnRequest
	opts.OnRequest = func(req *http.Request) error {
		if origOnReq != nil {
			if err := origOnReq(req); err != nil {
				return err
			}
		}
		return c.runRequestMiddlewarePlugins(req)
	}

	if headers, ok := msg["headers"].(map[string]any); ok {
		for name, value := range headers {
			if text, ok := value.(string); ok && text != "" {
				opts.Headers = append(opts.Headers, name+": "+text)
			}
		}
	}

	var reqBody io.Reader
	if bodyVal, ok := msg["body"]; ok && bodyVal != nil {
		ct, _ := msg["content_type"].(string)
		if ct == "" {
			ct = "application/json"
		}
		mimeType := c.content.MIMETypeForName(ct)
		if mimeType == "" {
			mimeType = ct
		}
		encoded, actualContentType, err := c.content.EncodeWithType(mimeType, bodyVal)
		if err != nil {
			reply := map[string]any{"type": "http-response", "error": err.Error()}
			return writer.WriteMessage(reply)
		}
		opts.Headers = append(opts.Headers, "Content-Type: "+actualContentType)
		reqBody = bytes.NewReader(encoded)
	}

	httpResp, err := request.Do(context.Background(), method, rawURL, reqBody, opts)
	if err != nil {
		reply := map[string]any{"type": "http-response", "error": err.Error()}
		return writer.WriteMessage(reply)
	}
	defer httpResp.Body.Close()

	resp, err := output.Normalize(httpResp, c.content)
	if err != nil {
		reply := map[string]any{"type": "http-response", "error": err.Error()}
		return writer.WriteMessage(reply)
	}

	reply := map[string]any{
		"type":    "http-response",
		"status":  resp.Status,
		"headers": resp.Headers,
		"body":    resp.Body,
	}
	return writer.WriteMessage(reply)
}

func (c *CLI) handlePluginResponse(cmd *cobra.Command, msg map[string]any) error {
	status := msgInt(msg, "status")
	if status == 0 {
		status = 200
	}
	resp := &output.Response{
		Proto:  "HTTP/1.1",
		Status: status,
		Body:   msg["body"],
	}
	if h, ok := msg["headers"].(map[string]any); ok {
		resp.Headers = make(map[string]string, len(h))
		for k, v := range h {
			if s, ok := v.(string); ok {
				resp.Headers[k] = s
			}
		}
	}
	return c.formatResponse(cmd, resp)
}

func (c *CLI) handlePluginAPISpec(writer *commandPluginWriter, msg map[string]any) error {
	apiName, _ := msg["name"].(string)
	if apiName == "" {
		return writer.WriteMessage(map[string]any{
			"type":  "api-spec-response",
			"error": "missing api name",
		})
	}
	if c.cfg == nil || c.cfg.APIs == nil || c.cfg.APIs[apiName] == nil {
		return writer.WriteMessage(map[string]any{
			"type":  "api-spec-response",
			"name":  apiName,
			"error": fmt.Sprintf("unknown API %q", apiName),
		})
	}

	s, err := spec.LoadFromCache(c.specCacheDir(), apiName, Version, c.loaders)
	if err != nil {
		return writer.WriteMessage(map[string]any{
			"type":  "api-spec-response",
			"name":  apiName,
			"error": err.Error(),
		})
	}
	if s == nil {
		s, err = c.discoverSpec(context.Background(), apiName)
		if err != nil {
			return writer.WriteMessage(map[string]any{
				"type":  "api-spec-response",
				"name":  apiName,
				"error": err.Error(),
			})
		}
	}
	if s == nil || len(s.Raw) == 0 {
		return writer.WriteMessage(map[string]any{
			"type":  "api-spec-response",
			"name":  apiName,
			"error": fmt.Sprintf("no spec available for %q", apiName),
		})
	}

	return writer.WriteMessage(map[string]any{
		"type":         "api-spec-response",
		"name":         apiName,
		"content_type": s.ContentType,
		"raw":          s.Raw,
	})
}

func msgInt(msg map[string]any, key string) int {
	v, ok := msg[key]
	if !ok {
		return 0
	}
	switch vt := v.(type) {
	case int64:
		return int(vt)
	case uint64:
		return int(vt)
	case float64:
		return int(vt)
	case int:
		return vt
	}
	return 0
}

func isEOFLike(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "EOF") || strings.Contains(s, "truncated") || strings.Contains(s, "broken pipe")
}

func msgBytes(v any) []byte {
	switch vt := v.(type) {
	case []byte:
		return vt
	case string:
		return []byte(vt)
	case []any:
		out := make([]byte, 0, len(vt))
		for _, item := range vt {
			switch b := item.(type) {
			case uint64:
				out = append(out, byte(b))
			case int64:
				out = append(out, byte(b))
			case int:
				out = append(out, byte(b))
			}
		}
		return out
	default:
		return nil
	}
}
