package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/danielgtaylor/restish/v2/plugin"
	"github.com/pb33f/libopenapi"
)

func main() {
	if plugin.HandleStartupFlags(os.Stdout, plugin.Manifest{
		Name:              "mcp",
		Version:           "1.0.0",
		Description:       "Expose registered APIs as MCP tools",
		RestishAPIVersion: 1,
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

	var initMsg map[string]any
	if err := plugin.ReadMessage(os.Stdin, &initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}
	command, _ := initMsg["command"].(string)
	if command != "mcp" {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone, ExitCode: 1})
		return
	}

	args := plugin.MsgStrings(initMsg["args"])
	client := newPluginClient(os.Stdin, os.Stdout)
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
	in           io.Reader
	out          io.Writer
	stdinPipeW   *io.PipeWriter
	stdinReader  io.Reader
	httpRespCh   chan map[string]any
	pendingStdin bytes.Buffer
	stdinClosed  bool
	writeMu      sync.Mutex
}

func newPluginClient(in io.Reader, out io.Writer) *pluginClient {
	pr, pw := io.Pipe()
	return &pluginClient{
		in:          in,
		out:         out,
		stdinPipeW:  pw,
		stdinReader: pr,
		httpRespCh:  make(chan map[string]any, 1),
	}
}

func (c *pluginClient) readLoop() {
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
		var msg map[string]any
		if err := plugin.ReadMessage(c.in, &msg); err != nil {
			return
		}
		switch msg["type"] {
		case plugin.MsgTypeStdinData:
			if data := plugin.MsgBytes(msg["data"]); len(data) > 0 {
				if _, err := c.stdinPipeW.Write(data); err != nil {
					return
				}
			}
		case plugin.MsgTypeStdinClose:
			_ = c.stdinPipeW.Close()
		case plugin.MsgTypeHTTPResponse:
			c.httpRespCh <- msg
		}
	}
}

func (c *pluginClient) do(req *HTTPRequest) (*HTTPResponse, error) {
	msg := plugin.HTTPRequestMsg{
		Type:   plugin.MsgTypeHTTPRequest,
		Method: req.Method,
		URI:    req.URI,
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
	reply, ok := <-c.httpRespCh
	if !ok {
		return nil, io.EOF
	}
	resp := &HTTPResponse{
		Status:  plugin.MsgInt(reply["status"]),
		Headers: mapAny(reply["headers"]),
		Body:    reply["body"],
	}
	if text, _ := reply["error"].(string); text != "" {
		resp.Error = text
	}
	return resp, nil
}

func (c *pluginClient) fetchSpecSync(name string) (*APISpec, error) {
	if err := c.writeMessage(plugin.APISpecMsg{Type: plugin.MsgTypeAPISpec, Name: name}); err != nil {
		return nil, err
	}
	var reply map[string]any
	for {
		if err := plugin.ReadMessage(c.in, &reply); err != nil {
			return nil, err
		}
		switch msgType, _ := reply["type"].(string); msgType {
		case plugin.MsgTypeAPISpecResponse:
			goto haveReply
		case plugin.MsgTypeStdinData:
			if data := plugin.MsgBytes(reply["data"]); len(data) > 0 {
				_, _ = c.pendingStdin.Write(data)
			}
		case plugin.MsgTypeStdinClose:
			c.stdinClosed = true
		default:
			return nil, fmt.Errorf("unexpected plugin reply %q while loading %s", msgType, name)
		}
	}
haveReply:
	if text, _ := reply["error"].(string); text != "" {
		return nil, fmt.Errorf("%s", text)
	}
	raw := plugin.MsgBytes(reply["raw"])
	if len(raw) == 0 {
		return nil, fmt.Errorf("api %q returned an empty spec", name)
	}
	doc, err := libopenapi.NewDocument(raw)
	if err != nil {
		return nil, fmt.Errorf("parse spec for %q: %w", name, err)
	}
	contentType, _ := reply["content_type"].(string)
	return &APISpec{
		Name:        name,
		ContentType: contentType,
		Raw:         raw,
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


func mapAny(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}
