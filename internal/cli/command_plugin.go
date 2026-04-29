package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/spec"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
	"github.com/spf13/cobra"
)

func loadCommandPluginCommands(path string) ([]pluginwire.CommandDecl, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, pluginwire.StartupFlagCommands)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plugin %s: command discovery: %w", filepath.Base(path), err)
	}
	if len(out) == 0 {
		return nil, nil
	}

	var resp struct {
		Commands []pluginwire.CommandDecl `cbor:"commands"`
	}
	if err := pluginwire.DecMode.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("plugin %s: commands decode: %w", filepath.Base(path), err)
	}
	return resp.Commands, nil
}

func (c *CLI) addCommandPlugins(root *cobra.Command) {
	seen := map[string]string{}
	for _, p := range c.pluginsByHook["command"] {
		cmds, err := loadCommandPluginCommands(p.Path)
		if err != nil {
			c.warnf("plugin %s: %v", filepath.Base(p.Path), err)
			continue
		}
		if len(cmds) == 0 {
			continue
		}
		for _, decl := range cmds {
			decl := decl
			pluginPath := p.Path
			if err := c.validatePluginCommandName(root, seen, filepath.Base(pluginPath), decl.Name); err != nil {
				c.warnf("%v", err)
				continue
			}
			seen[decl.Name] = filepath.Base(pluginPath)
			root.AddCommand(&cobra.Command{
				Use:                decl.Name,
				Short:              decl.Short,
				Long:               decl.Long,
				GroupID:            rootGroupPlugin,
				Args:               cobra.ArbitraryArgs,
				DisableFlagParsing: true,
				RunE: func(cmd *cobra.Command, args []string) error {
					return c.runCommandPlugin(cmd, pluginPath, decl, args)
				},
			})
		}
	}
}

var pluginCommandNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func (c *CLI) validatePluginCommandName(root *cobra.Command, seen map[string]string, pluginName, name string) error {
	if !pluginCommandNamePattern.MatchString(name) {
		return fmt.Errorf("plugin %s: command name %q is invalid; use lower-case letters, digits, and dashes", pluginName, name)
	}
	if rootHasCommand(root, name) || isBuiltinCommandName(name) {
		return fmt.Errorf("plugin %s: command %q collides with a built-in command; skipping", pluginName, name)
	}
	if c.cfg != nil && c.cfg.APIs[name] != nil {
		return fmt.Errorf("plugin %s: command %q collides with a registered API; skipping", pluginName, name)
	}
	if previous := seen[name]; previous != "" {
		return fmt.Errorf("plugin %s: command %q duplicates command from plugin %s; skipping", pluginName, name, previous)
	}
	return nil
}

type commandPluginWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (w *commandPluginWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.w.Write(p)
}

func (w *commandPluginWriter) WriteMessage(v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return pluginwire.WriteMessage(w.w, v)
}

func (c *CLI) runCommandPlugin(cmd *cobra.Command, pluginPath string, decl pluginwire.CommandDecl, args []string) error {
	syncErr := &commandPluginWriter{w: cmd.ErrOrStderr()}
	cmd.SetErr(syncErr)

	proc := exec.CommandContext(cmd.Context(), pluginPath, append(terminalContextFlags(c), args...)...)
	proc.Stderr = cmd.ErrOrStderr()

	stdinPipe, err := proc.StdinPipe()
	if err != nil {
		return fmt.Errorf("command plugin: stdin pipe: %w", err)
	}
	stdoutPipe, err := proc.StdoutPipe()
	if err != nil {
		_ = stdinPipe.Close()
		return fmt.Errorf("command plugin: stdout pipe: %w", err)
	}
	if err := proc.Start(); err != nil {
		_ = stdinPipe.Close()
		_ = stdoutPipe.Close()
		return fmt.Errorf("command plugin: start: %w", err)
	}
	cancelWatchDone := make(chan struct{})
	go func() {
		select {
		case <-cmd.Context().Done():
			_ = stdinPipe.Close()
			_ = stdoutPipe.Close()
		case <-cancelWatchDone:
		}
	}()
	cleanupStartFailure := func(cause error) error {
		close(cancelWatchDone)
		_ = stdinPipe.Close()
		_ = stdoutPipe.Close()
		if proc.Process != nil {
			_ = proc.Process.Kill()
		}
		_ = proc.Wait()
		return cause
	}

	writer := &commandPluginWriter{w: stdinPipe}
	initMsg := pluginwire.InitMsg{
		Type:    pluginwire.MsgTypeInit,
		Command: decl.Name,
		Args:    args,
	}
	if err := writer.WriteMessage(initMsg); err != nil {
		return cleanupStartFailure(fmt.Errorf("command plugin: send init: %w", err))
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
	doneReceived := false
	var requestWG sync.WaitGroup
	dec := pluginwire.NewDecoder(stdoutPipe)
	for {
		raw, err := dec.ReadRaw()
		if err != nil {
			if ctxErr := cmd.Context().Err(); ctxErr != nil {
				loopErr = ctxErr
			} else if isEOFLike(err) {
				loopErr = fmt.Errorf("command plugin %s: process died unexpectedly", filepath.Base(pluginPath))
			} else {
				loopErr = fmt.Errorf("command plugin %s: read message: %w", filepath.Base(pluginPath), err)
			}
			break
		}

		done, err := c.handleCommandPluginMessage(cmd, writer, &requestWG, pluginwire.MessageType(raw), raw)
		if err != nil {
			loopErr = err
			break
		}
		if done {
			doneReceived = pluginwire.MessageType(raw) == pluginwire.MsgTypeDone
			break
		}
	}

	close(cancelWatchDone)
	close(stopCh)
	if loopErr != nil {
		_ = stdinPipe.Close()
		requestWG.Wait()
	} else {
		requestWG.Wait()
		_ = stdinPipe.Close()
	}
	waitErr := waitCommandPluginExit(proc, commandPluginShutdownGrace())
	if loopErr == nil && waitErr != nil && !doneReceived {
		loopErr = fmt.Errorf("command plugin %s: wait: %w", filepath.Base(pluginPath), waitErr)
	}
	// Warn when the plugin exits non-zero after sending Done — this is not a CLI
	// error but may indicate a plugin bug (e.g. panic in a deferred cleanup).
	if doneReceived && waitErr != nil {
		c.warnf("command plugin %s exited with error after Done: %v", filepath.Base(pluginPath), waitErr)
	}
	return loopErr
}

func waitCommandPluginExit(proc *exec.Cmd, grace time.Duration) error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- proc.Wait()
	}()

	timer := time.NewTimer(grace)
	defer timer.Stop()

	select {
	case err := <-waitCh:
		return err
	case <-timer.C:
		if proc.Process != nil {
			_ = proc.Process.Kill()
		}
		return <-waitCh
	}
}

func commandPluginShutdownGrace() time.Duration {
	if value := strings.TrimSpace(os.Getenv("RSH_COMMAND_PLUGIN_SHUTDOWN_GRACE")); value != "" {
		if d, err := time.ParseDuration(value); err == nil && d > 0 {
			return d
		}
	}
	return 5 * time.Second
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
	if stdin, ok := c.Stdin.(*os.File); ok && stdin != os.Stdin {
		go func() {
			<-done
			_ = stdin.Close()
		}()
	}

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
				if writeErr := writer.WriteMessage(pluginwire.StdinDataMsg{
					Type: pluginwire.MsgTypeStdinData,
					Data: r.data,
				}); writeErr != nil {
					return
				}
			}
			if errors.Is(r.err, io.EOF) {
				_ = writer.WriteMessage(pluginwire.StdinCloseMsg{Type: pluginwire.MsgTypeStdinClose})
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

func (c *CLI) handleCommandPluginMessage(cmd *cobra.Command, writer *commandPluginWriter, requestWG *sync.WaitGroup, msgType string, raw []byte) (bool, error) {
	switch msgType {
	case pluginwire.MsgTypeDone:
		var msg pluginwire.DoneMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return true, err
		}
		if msg.ExitCode != 0 {
			return true, &ExitCodeError{Code: msg.ExitCode}
		}
		return true, nil
	case pluginwire.MsgTypeHTTPRequest:
		var msg pluginwire.HTTPRequestMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		requestWG.Add(1)
		go func() {
			defer requestWG.Done()
			if err := c.handlePluginHTTPRequest(cmd, writer, msg); err != nil {
				_ = writer.WriteMessage(pluginwire.HTTPResponseMsg{
					Type:      pluginwire.MsgTypeHTTPResponse,
					RequestID: msg.RequestID,
					Error:     err.Error(),
				})
			}
		}()
		return false, nil
	case pluginwire.MsgTypeAPISpec:
		var msg pluginwire.APISpecMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		return false, c.handlePluginAPISpec(cmd.Context(), writer, msg)
	case pluginwire.MsgTypeListAPIs:
		return false, c.handlePluginListAPIs(writer)
	case pluginwire.MsgTypeListProfiles:
		var msg pluginwire.ListProfilesMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		return false, c.handlePluginListProfiles(writer, msg)
	case pluginwire.MsgTypeConfigRead:
		var msg pluginwire.ConfigReadMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		return false, c.handlePluginConfigRead(writer, msg)
	case pluginwire.MsgTypePrompt:
		var msg pluginwire.PromptMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		return false, c.handlePluginPrompt(cmd.Context(), writer, msg)
	case pluginwire.MsgTypeConfirm:
		var msg pluginwire.ConfirmMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		return false, c.handlePluginConfirm(cmd.Context(), writer, msg)
	case pluginwire.MsgTypeResponse:
		var msg pluginwire.ResponseMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		return false, c.handlePluginResponse(cmd, msg)
	case pluginwire.MsgTypeStdoutData:
		var msg pluginwire.StdoutDataMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		if len(msg.Data) > 0 {
			_, _ = c.Stdout.Write(msg.Data)
		}
	case pluginwire.MsgTypeStderrData:
		var msg pluginwire.StderrDataMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		if len(msg.Data) > 0 {
			_, _ = cmd.ErrOrStderr().Write(msg.Data)
		}
	case pluginwire.MsgTypeWarn:
		var msg pluginwire.WarnMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		if msg.Text != "" {
			writeDiagnostic(cmd.ErrOrStderr(), diagnosticWarn, "warning", "%s", msg.Text)
		}
	case pluginwire.MsgTypeProgress:
		var msg pluginwire.ProgressMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		if msg.Text != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), msg.Text)
		}
	case pluginwire.MsgTypeSpinner:
		var msg pluginwire.SpinnerMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		if msg.Text != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), msg.Text)
		}
	case pluginwire.MsgTypeLog:
		var msg pluginwire.LogMsg
		if err := decodeCommandPluginMessage(msgType, raw, &msg); err != nil {
			return false, err
		}
		if msg.Text != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), msg.Text)
		}
	default:
		if msgType != "" {
			writeDiagnostic(cmd.ErrOrStderr(), diagnosticWarn, "warning", "unhandled plugin message type %q", msgType)
		}
	}
	return false, nil
}

func decodeCommandPluginMessage(msgType string, raw []byte, dst any) error {
	if err := pluginwire.DecMode.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("command plugin: decode %s: %w", msgType, err)
	}
	return nil
}

func (c *CLI) handlePluginHTTPRequest(cmd *cobra.Command, writer *commandPluginWriter, msg pluginwire.HTTPRequestMsg) error {
	method := msg.Method
	if method == "" {
		method = "GET"
	}

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	profileName := c.profileFromCmd(cmd)

	if msg.NoCache {
		opts.NoCache = true
	}
	if msg.CacheTTL > 0 {
		opts.Headers = append(opts.Headers, fmt.Sprintf("Cache-Control: max-age=%d", msg.CacheTTL))
	}
	for name, value := range msg.Headers {
		if value != "" {
			opts.Headers = append(opts.Headers, name+": "+value)
		}
	}
	if msg.Body != nil {
		opts.ContentType = msg.ContentType
	}

	reqCtx := cmd.Context()
	if msg.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(cmd.Context(), time.Duration(msg.Timeout)*time.Second)
		defer cancel()
	}

	prepared, err := c.prepareRequest(msg.URI, profileName, opts, msg.Body, nil, false, authHandlerOptions{}, nil)
	if err != nil {
		return writer.WriteMessage(pluginwire.HTTPResponseMsg{
			Type:      pluginwire.MsgTypeHTTPResponse,
			RequestID: msg.RequestID,
			Error:     err.Error(),
		})
	}
	defer c.closePreparedTransport(prepared)

	httpResp, err := c.sendPreparedRequest(reqCtx, method, prepared)
	if err != nil {
		return writer.WriteMessage(pluginwire.HTTPResponseMsg{
			Type:      pluginwire.MsgTypeHTTPResponse,
			RequestID: msg.RequestID,
			Error:     err.Error(),
		})
	}

	resp, err := c.normalizeHTTPResponse(httpResp, maxBodyBytes(cmd))
	if err != nil {
		return writer.WriteMessage(pluginwire.HTTPResponseMsg{
			Type:      pluginwire.MsgTypeHTTPResponse,
			RequestID: msg.RequestID,
			Error:     err.Error(),
		})
	}

	body := resp.Body
	if msg.Filter != "" {
		doc := map[string]any{
			"proto":   resp.Proto,
			"status":  resp.Status,
			"headers": resp.Headers,
			"links":   resp.Links,
			"body":    resp.Body,
		}
		filtered, ferr := filter.Apply(msg.Filter, doc, filter.LangAuto)
		if ferr != nil {
			return writer.WriteMessage(pluginwire.HTTPResponseMsg{
				Type:      pluginwire.MsgTypeHTTPResponse,
				RequestID: msg.RequestID,
				Error:     ferr.Error(),
			})
		}
		body = filtered
	}

	return writer.WriteMessage(pluginwire.HTTPResponseMsg{
		Type:      pluginwire.MsgTypeHTTPResponse,
		RequestID: msg.RequestID,
		Status:    resp.Status,
		Headers:   resp.Headers,
		Body:      body,
	})
}

func (c *CLI) handlePluginResponse(cmd *cobra.Command, msg pluginwire.ResponseMsg) error {
	status := msg.Status
	if status == 0 {
		status = 200
	}
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  status,
		Headers: msg.Headers,
		Body:    msg.Body,
	}
	return c.formatResponse(cmd, resp)
}

func (c *CLI) handlePluginAPISpec(ctx context.Context, writer *commandPluginWriter, msg pluginwire.APISpecMsg) error {
	if msg.Name == "" {
		return writer.WriteMessage(pluginwire.APISpecResponseMsg{
			Type:  pluginwire.MsgTypeAPISpecResponse,
			Error: "missing api name",
		})
	}
	if c.cfg == nil || c.cfg.APIs == nil || c.cfg.APIs[msg.Name] == nil {
		return writer.WriteMessage(pluginwire.APISpecResponseMsg{
			Type:  pluginwire.MsgTypeAPISpecResponse,
			Name:  msg.Name,
			Error: fmt.Sprintf("unknown API %q", msg.Name),
		})
	}
	apiCfg := c.cfg.APIs[msg.Name]

	s, err := spec.LoadFromCache(c.specCacheDir(), msg.Name, Version, apiCfg.SpecFiles, c.loaders)
	if err != nil {
		return writer.WriteMessage(pluginwire.APISpecResponseMsg{
			Type:  pluginwire.MsgTypeAPISpecResponse,
			Name:  msg.Name,
			Error: err.Error(),
		})
	}
	if s == nil {
		s, err = c.discoverSpec(ctx, msg.Name)
		if err != nil {
			return writer.WriteMessage(pluginwire.APISpecResponseMsg{
				Type:  pluginwire.MsgTypeAPISpecResponse,
				Name:  msg.Name,
				Error: err.Error(),
			})
		}
	}
	if s == nil || len(s.Raw) == 0 {
		return writer.WriteMessage(pluginwire.APISpecResponseMsg{
			Type:  pluginwire.MsgTypeAPISpecResponse,
			Name:  msg.Name,
			Error: fmt.Sprintf("no spec available for %q", msg.Name),
		})
	}
	opSet, err := s.OperationSetWithOptions(spec.OperationOptions{
		BaseURL:         apiCfg.BaseURL,
		OperationBase:   apiCfg.OperationBase,
		ServerVariables: effectiveServerVariables(apiCfg, "default"),
	})
	if err != nil {
		return writer.WriteMessage(pluginwire.APISpecResponseMsg{
			Type:  pluginwire.MsgTypeAPISpecResponse,
			Name:  msg.Name,
			Error: err.Error(),
		})
	}

	return writer.WriteMessage(pluginwire.APISpecResponseMsg{
		Type:        pluginwire.MsgTypeAPISpecResponse,
		Name:        msg.Name,
		ContentType: s.ContentType,
		Raw:         s.Raw,
		Operations:  pluginOperationsFromSpec(opSet.Operations),
	})
}

func pluginOperationsFromSpec(ops []spec.Operation) []pluginwire.APIOperation {
	if ops == nil {
		return nil
	}
	out := make([]pluginwire.APIOperation, 0, len(ops))
	for _, op := range ops {
		params := make([]pluginwire.APIParam, 0, len(op.Parameters))
		for _, p := range op.Parameters {
			if p.XCLI.Ignore {
				continue
			}
			params = append(params, pluginwire.APIParam{
				Name:        p.Name,
				In:          p.In,
				Required:    p.Required,
				Description: p.Desc,
				Type:        p.Type,
				ItemType:    p.ItemType,
				Enum:        append([]string(nil), p.Enum...),
			})
		}
		out = append(out, pluginwire.APIOperation{
			ID:               op.ID,
			Method:           op.Method,
			Path:             op.Path,
			Summary:          op.Summary,
			Description:      op.Description,
			Deprecated:       op.Deprecated,
			Parameters:       params,
			HasBody:          op.HasBody,
			BodyRequired:     op.BodyRequired,
			RequestMediaType: op.RequestMediaType,
			MCPIgnore:        op.MCPIgnore,
		})
	}
	return out
}

func (c *CLI) handlePluginPrompt(ctx context.Context, writer *commandPluginWriter, msg pluginwire.PromptMsg) error {
	var value string
	var readErr error
	if msg.Hidden {
		value, readErr = c.Secret(ctx, msg.Message)
	} else {
		value, readErr = c.Prompt(ctx, msg.Message)
	}
	if readErr != nil {
		return writer.WriteMessage(pluginwire.PromptResponseMsg{
			Type:  pluginwire.MsgTypePromptResponse,
			Error: readErr.Error(),
		})
	}
	return writer.WriteMessage(pluginwire.PromptResponseMsg{
		Type:  pluginwire.MsgTypePromptResponse,
		Value: value,
	})
}

func (c *CLI) handlePluginConfirm(ctx context.Context, writer *commandPluginWriter, msg pluginwire.ConfirmMsg) error {
	confirmed, err := c.Confirm(ctx, msg.Message)
	if err != nil {
		return writer.WriteMessage(pluginwire.ConfirmResponseMsg{
			Type:  pluginwire.MsgTypeConfirmResponse,
			Error: err.Error(),
		})
	}
	return writer.WriteMessage(pluginwire.ConfirmResponseMsg{
		Type:  pluginwire.MsgTypeConfirmResponse,
		Value: confirmed,
	})
}

func (c *CLI) handlePluginConfigRead(writer *commandPluginWriter, msg pluginwire.ConfigReadMsg) error {
	reply := pluginwire.ConfigReadResponseMsg{Type: pluginwire.MsgTypeConfigReadResponse}

	if msg.Plugin != "" && c.cfg != nil && c.cfg.Plugins != nil {
		if raw, ok := c.cfg.Plugins[msg.Plugin]; ok {
			var pluginCfg any
			if err := json.Unmarshal(raw, &pluginCfg); err == nil {
				reply.PluginConfig = pluginCfg
			}
		}
	}

	if c.cfg == nil || msg.API == "" {
		return writer.WriteMessage(reply)
	}
	apiCfg := c.cfg.APIs[msg.API]
	if apiCfg == nil {
		reply.Error = fmt.Sprintf("unknown API %q", msg.API)
		return writer.WriteMessage(reply)
	}
	baseURL := apiCfg.BaseURL
	if msg.Profile != "" {
		if prof := apiCfg.Profiles[msg.Profile]; prof != nil {
			if prof.BaseURL != "" {
				baseURL = prof.BaseURL
			}
			reply.Headers = prof.Headers
			reply.Query = prof.Query
		}
	}
	reply.BaseURL = baseURL
	return writer.WriteMessage(reply)
}

func (c *CLI) handlePluginListAPIs(writer *commandPluginWriter) error {
	var names []string
	if c.cfg != nil {
		names = make([]string, 0, len(c.cfg.APIs))
		for name := range c.cfg.APIs {
			names = append(names, name)
		}
		sort.Strings(names)
	}
	return writer.WriteMessage(pluginwire.ListAPIsResponseMsg{
		Type: pluginwire.MsgTypeListAPIsResponse,
		APIs: names,
	})
}

func (c *CLI) handlePluginListProfiles(writer *commandPluginWriter, msg pluginwire.ListProfilesMsg) error {
	var profileNames []string
	if c.cfg != nil && msg.API != "" {
		if apiCfg := c.cfg.APIs[msg.API]; apiCfg != nil {
			profileNames = make([]string, 0, len(apiCfg.Profiles))
			for name := range apiCfg.Profiles {
				profileNames = append(profileNames, name)
			}
			sort.Strings(profileNames)
		}
	}
	return writer.WriteMessage(pluginwire.ListProfilesResponseMsg{
		Type:     pluginwire.MsgTypeListProfilesResponse,
		API:      msg.API,
		Profiles: profileNames,
	})
}

func isEOFLike(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	// CBOR decoder wraps io.EOF/io.ErrUnexpectedEOF; catch any remaining cases
	// via string matching as a fallback for library-specific error types.
	s := err.Error()
	return strings.Contains(s, "EOF") || strings.Contains(s, "truncated") || strings.Contains(s, "broken pipe")
}
