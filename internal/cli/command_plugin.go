package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
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
	Name  string `cbor:"name" json:"name"`
	Short string `cbor:"short" json:"short"`
	Long  string `cbor:"long" json:"long"`
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
					return c.runCommandPlugin(cmd, pluginPath, decl.Name, args)
				},
			})
		}
	}
}

func (c *CLI) runCommandPlugin(cmd *cobra.Command, pluginPath, commandName string, args []string) error {
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

	initMsg := map[string]any{
		"type":    "init",
		"command": commandName,
		"args":    args,
	}
	if err := pluginwire.WriteMessage(stdinPipe, initMsg); err != nil {
		_ = proc.Process.Kill()
		return fmt.Errorf("command plugin: send init: %w", err)
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

		done, err := c.handleCommandPluginMessage(cmd, stdinPipe, msg)
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

func (c *CLI) handleCommandPluginMessage(cmd *cobra.Command, stdinPipe io.Writer, msg map[string]any) (bool, error) {
	msgType, _ := msg["type"].(string)
	switch msgType {
	case "done":
		code := msgInt(msg, "exit_code")
		if code != 0 {
			return true, &ExitCodeError{Code: code}
		}
		return true, nil
	case "http-request":
		return false, c.handlePluginHTTPRequest(cmd, stdinPipe, msg)
	case "response":
		return false, c.handlePluginResponse(cmd, msg)
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

func (c *CLI) handlePluginHTTPRequest(cmd *cobra.Command, stdinPipe io.Writer, msg map[string]any) error {
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

	httpResp, err := request.Do(context.Background(), method, rawURL, nil, opts)
	if err != nil {
		reply := map[string]any{"type": "http-response", "error": err.Error()}
		return pluginwire.WriteMessage(stdinPipe, reply)
	}
	defer httpResp.Body.Close()

	resp, err := output.Normalize(httpResp, c.content)
	if err != nil {
		reply := map[string]any{"type": "http-response", "error": err.Error()}
		return pluginwire.WriteMessage(stdinPipe, reply)
	}

	reply := map[string]any{
		"type":    "http-response",
		"status":  resp.Status,
		"headers": resp.Headers,
		"body":    resp.Body,
	}
	return pluginwire.WriteMessage(stdinPipe, reply)
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
