package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/output"
	internalplugin "github.com/danielgtaylor/restish/v2/internal/plugin"
	"github.com/danielgtaylor/restish/v2/internal/request"
	"github.com/danielgtaylor/restish/v2/internal/spec"
	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
	"github.com/spf13/cobra"
)

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
	if err := pluginwire.DecMode.Unmarshal(out, &resp); err != nil {
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
				Use:                decl.Name,
				Short:              decl.Short,
				Long:               decl.Long,
				Args:               cobra.ArbitraryArgs,
				DisableFlagParsing: true,
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

	// stopCh is closed when the command loop exits, signalling streamPluginStdin
	// to stop forwarding. For TTY stdin the inner reader goroutine may remain
	// briefly blocked until the user interacts, but it will exit promptly once
	// stdinPipe is closed and the next write fails.
	stopCh := make(chan struct{})
	if decl.PassthroughStdio {
		go c.streamPluginStdin(writer, stopCh)
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

	close(stopCh)
	_ = stdinPipe.Close()
	_ = proc.Wait()
	return loopErr
}

// streamPluginStdin forwards c.Stdin to the command plugin as "stdin-data"
// messages until stdin closes, a write error occurs, or done is closed.
//
// An inner goroutine performs the blocking Read from c.Stdin so that the outer
// goroutine can select on both the read result and the done signal. When done
// is closed the outer goroutine exits immediately; the inner goroutine remains
// alive only until c.Stdin yields its next byte (TTY) or closes (pipe), at
// which point it exits through the done-guarded channel send.
func (c *CLI) streamPluginStdin(writer *commandPluginWriter, done <-chan struct{}) {
	type chunk struct {
		data []byte
		err  error
	}
	reads := make(chan chunk, 4)

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := c.Stdin.Read(buf)
			var data []byte
			if n > 0 {
				data = make([]byte, n)
				copy(data, buf[:n])
			}
			select {
			case reads <- chunk{data: data, err: err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case r := <-reads:
			if len(r.data) > 0 {
				if writeErr := writer.WriteMessage(map[string]any{
					"type": "stdin-data",
					"data": r.data,
				}); writeErr != nil {
					return
				}
			}
			if r.err == io.EOF {
				_ = writer.WriteMessage(map[string]any{"type": "stdin-close"})
				return
			}
			if r.err != nil {
				return
			}
		case <-done:
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
		if data := internalplugin.MsgBytes(msg["data"]); len(data) > 0 {
			_, _ = c.Stdout.Write(data)
		}
	case "stderr-data":
		if data := internalplugin.MsgBytes(msg["data"]); len(data) > 0 {
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
	default:
		if msgType != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: unhandled plugin message type %q\n", msgType)
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

	profileName := c.profileFromCmd(cmd)
	rawURL, _, opts = c.applyAPIProfile(rawURL, profileName, opts)
	opts, err = c.resolveTLSSigner(opts)
	if err != nil {
		reply := map[string]any{"type": "http-response", "error": err.Error()}
		return writer.WriteMessage(reply)
	}
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

	resp, err := output.Normalize(httpResp, c.content, maxBodyBytes(cmd))
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
