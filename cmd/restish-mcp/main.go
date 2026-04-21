package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/rest-sh/restish/v2/plugin"
	"github.com/pb33f/libopenapi"
)

func main() {
	if plugin.HandleStartupFlags(os.Stdout, plugin.Manifest{
		Name:              "mcp",
		Version:           "1.0.0",
		Description:       "Expose registered APIs as MCP tools",
		RestishAPIVersion: 2,
		Hooks:             []string{"command"},
	}, []plugin.CommandDecl{
		{
			Name:             "mcp",
			Short:            "Serve registered APIs over the Model Context Protocol",
			Long:             "Expose OpenAPI operations as MCP tools via Restish-authenticated HTTP delegation.",
			PassthroughStdio: true,
		},
	}) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)
	var initMsg plugin.InitMsg
	if err := dec.ReadMessage(&initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}
	if initMsg.Command != "mcp" {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", initMsg.Command)
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone, ExitCode: 1})
		return
	}

	args := initMsg.Args
	client := newPluginClient(dec, os.Stdout)
	cfg, err := ParseArgs(args)
	if err == nil {
		var tools []*Tool
		tools, err = LoadTools(client.fetchSpecSync, cfg.APINames, cfg.Options)
		if err == nil {
			go client.readLoop()
			server := &Server{
				Tools:          tools,
				ToolIndex:      indexTools(tools),
				Exec:           client.do,
				MaxResultBytes: cfg.Options.MaxResultBytes,
			}
			if server.MaxResultBytes <= 0 {
				server.MaxResultBytes = DefaultMaxResultBytes
			}
			err = server.ServeStdio(client.stdinReader, client.newStdoutWriter())
		}
	}
	if err != nil {
		_ = plugin.WriteMessage(os.Stdout, plugin.StderrDataMsg{Type: plugin.MsgTypeStderrData, Data: []byte(err.Error() + "\n")})
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone, ExitCode: 1})
		return
	}
	_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone})
}

type pluginClient struct {
	dec          *plugin.Decoder
	out          io.Writer
	stdinPipeW   *io.PipeWriter
	stdinReader  io.Reader
	httpRespCh   chan plugin.HTTPResponseMsg
	nextHTTPID   uint64
	pendingMu    sync.Mutex
	pendingHTTP  map[string]chan plugin.HTTPResponseMsg
	pendingStdin bytes.Buffer
	stdinClosed  bool
	writeMu      sync.Mutex
}

func newPluginClient(dec *plugin.Decoder, out io.Writer) *pluginClient {
	pr, pw := io.Pipe()
	return &pluginClient{
		dec:         dec,
		out:         out,
		stdinPipeW:  pw,
		stdinReader: pr,
		httpRespCh:  make(chan plugin.HTTPResponseMsg, 1),
		pendingHTTP: map[string]chan plugin.HTTPResponseMsg{},
	}
}

func (c *pluginClient) readLoop() {
	defer close(c.httpRespCh)
	defer c.closePendingHTTP()
	defer c.stdinPipeW.Close()
	if c.pendingStdin.Len() > 0 {
		if _, err := io.Copy(c.stdinPipeW, &c.pendingStdin); err != nil {
			return
		}
	}
	if c.stdinClosed {
		_ = c.stdinPipeW.Close()
	}
	for {
		raw, err := c.dec.ReadRaw()
		if err != nil {
			return
		}
		switch plugin.MessageType(raw) {
		case plugin.MsgTypeStdinData:
			var msg plugin.StdinDataMsg
			if err := plugin.DecMode.Unmarshal(raw, &msg); err == nil && len(msg.Data) > 0 {
				if _, err := c.stdinPipeW.Write(msg.Data); err != nil {
					return
				}
			}
		case plugin.MsgTypeStdinClose:
			_ = c.stdinPipeW.Close()
		case plugin.MsgTypeHTTPResponse:
			var msg plugin.HTTPResponseMsg
			if err := plugin.DecMode.Unmarshal(raw, &msg); err == nil {
				if msg.RequestID != "" {
					if c.sendPendingHTTP(msg.RequestID, msg) {
						continue
					}
				}
				select {
				case c.httpRespCh <- msg:
				default:
				}
			}
		}
	}
}

func (c *pluginClient) do(req *HTTPRequest) (*HTTPResponse, error) {
	requestID := strconv.FormatUint(atomic.AddUint64(&c.nextHTTPID, 1), 10)
	replyCh := c.registerPendingHTTP(requestID)
	defer c.unregisterPendingHTTP(requestID)

	msg := plugin.HTTPRequestMsg{
		Type:      plugin.MsgTypeHTTPRequest,
		RequestID: requestID,
		Method:    req.Method,
		URI:       req.URI,
	}
	if len(req.Headers) > 0 {
		msg.Headers = req.Headers
	}
	if req.Body != nil {
		msg.Body = req.Body
	}
	if req.ContentType != "" {
		msg.ContentType = req.ContentType
	}
	if err := c.writeMessage(msg); err != nil {
		return nil, err
	}
	var (
		reply plugin.HTTPResponseMsg
		ok    bool
	)
	select {
	case reply, ok = <-replyCh:
	case reply, ok = <-c.httpRespCh:
	}
	if !ok {
		return nil, io.EOF
	}
	return &HTTPResponse{
		Status:  reply.Status,
		Headers: reply.Headers,
		Body:    reply.Body,
		Error:   reply.Error,
	}, nil
}

func (c *pluginClient) fetchSpecSync(name string) (*APISpec, error) {
	if err := c.writeMessage(plugin.APISpecMsg{Type: plugin.MsgTypeAPISpec, Name: name}); err != nil {
		return nil, err
	}
	var reply plugin.APISpecResponseMsg
	for {
		raw, err := c.dec.ReadRaw()
		if err != nil {
			return nil, err
		}
		switch plugin.MessageType(raw) {
		case plugin.MsgTypeAPISpecResponse:
			if err := plugin.DecMode.Unmarshal(raw, &reply); err != nil {
				return nil, fmt.Errorf("decode api spec response: %w", err)
			}
			goto haveReply
		case plugin.MsgTypeStdinData:
			var msg plugin.StdinDataMsg
			if err := plugin.DecMode.Unmarshal(raw, &msg); err == nil && len(msg.Data) > 0 {
				_, _ = c.pendingStdin.Write(msg.Data)
			}
		case plugin.MsgTypeStdinClose:
			c.stdinClosed = true
		default:
			return nil, fmt.Errorf("unexpected plugin reply %q while loading %s", plugin.MessageType(raw), name)
		}
	}
haveReply:
	if reply.Error != "" {
		return nil, fmt.Errorf("%s", reply.Error)
	}
	if len(reply.Raw) == 0 {
		return nil, fmt.Errorf("api %q returned an empty spec", name)
	}
	doc, err := libopenapi.NewDocument(reply.Raw)
	if err != nil {
		return nil, fmt.Errorf("parse spec for %q: %w", name, err)
	}
	return &APISpec{
		Name:        name,
		ContentType: reply.ContentType,
		Raw:         reply.Raw,
		Document:    doc,
	}, nil
}

func (c *pluginClient) writeMessage(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return plugin.WriteMessage(c.out, v)
}

type stdoutWriter struct {
	client *pluginClient
}

func (w *stdoutWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := w.client.writeMessage(plugin.StdoutDataMsg{Type: plugin.MsgTypeStdoutData, Data: append([]byte(nil), p...)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *pluginClient) newStdoutWriter() io.Writer {
	return &stdoutWriter{client: c}
}

func (c *pluginClient) registerPendingHTTP(requestID string) chan plugin.HTTPResponseMsg {
	replyCh := make(chan plugin.HTTPResponseMsg, 1)
	c.pendingMu.Lock()
	c.pendingHTTP[requestID] = replyCh
	c.pendingMu.Unlock()
	return replyCh
}

func (c *pluginClient) unregisterPendingHTTP(requestID string) {
	c.pendingMu.Lock()
	delete(c.pendingHTTP, requestID)
	c.pendingMu.Unlock()
}

func (c *pluginClient) sendPendingHTTP(requestID string, msg plugin.HTTPResponseMsg) bool {
	c.pendingMu.Lock()
	replyCh := c.pendingHTTP[requestID]
	c.pendingMu.Unlock()
	if replyCh == nil {
		return false
	}
	select {
	case replyCh <- msg:
	default:
	}
	return true
}

func (c *pluginClient) closePendingHTTP() {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	for requestID, replyCh := range c.pendingHTTP {
		close(replyCh)
		delete(c.pendingHTTP, requestID)
	}
}
