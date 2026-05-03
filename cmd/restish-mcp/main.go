package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pb33f/libopenapi"
	"github.com/rest-sh/restish/v2/plugin"
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
			Long:             "Expose OpenAPI operations as MCP tools via Restish-authenticated HTTP delegation.\n\nUsage:\n  restish mcp serve <api...>",
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
		var stats ToolLoadStats
		tools, stats, err = LoadToolsWithStats(client.fetchSpecSync, cfg.APINames, cfg.Options)
		if err == nil {
			if stats.HiddenWriteOperations > 0 {
				_ = plugin.WriteMessage(os.Stdout, plugin.StderrDataMsg{
					Type: plugin.MsgTypeStderrData,
					Data: []byte(fmt.Sprintf("mcp: hid %d write operation(s); pass --allow-write-tools to expose POST, PUT, PATCH, and DELETE\n", stats.HiddenWriteOperations)),
				})
			}
			server := &Server{
				Tools:          tools,
				ToolIndex:      indexTools(tools),
				Exec:           client.do,
				MaxResultBytes: cfg.Options.MaxResultBytes,
				RequestTimeout: cfg.Options.RequestTimeout,
			}
			if server.MaxResultBytes <= 0 {
				server.MaxResultBytes = DefaultMaxResultBytes
			}
			client.startStdinForwarding()
			err = server.ServeStdio(client.stdinReader, client.StdoutWriter())
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
	*plugin.CommandClient
	stdinPipeW   *io.PipeWriter
	stdinReader  io.Reader
	stdinMu      sync.Mutex
	stdinWriteMu sync.Mutex
	pendingStdin bytes.Buffer
	stdinClosed  bool
	stdinForward bool
}

func newPluginClient(dec *plugin.Decoder, out io.Writer) *pluginClient {
	pr, pw := io.Pipe()
	client := &pluginClient{
		CommandClient: plugin.NewCommandClientFromDecoder(dec, out),
		stdinPipeW:    pw,
		stdinReader:   pr,
	}
	client.StdinDataHandler = func(data []byte) {
		client.stdinMu.Lock()
		if !client.stdinForward {
			_, _ = client.pendingStdin.Write(data)
			client.stdinMu.Unlock()
			return
		}
		client.stdinMu.Unlock()
		client.stdinWriteMu.Lock()
		defer client.stdinWriteMu.Unlock()
		_, _ = client.stdinPipeW.Write(data)
	}
	client.StdinCloseHandler = func() {
		client.stdinMu.Lock()
		if !client.stdinForward {
			client.stdinClosed = true
			client.stdinMu.Unlock()
			return
		}
		client.stdinMu.Unlock()
		client.stdinWriteMu.Lock()
		defer client.stdinWriteMu.Unlock()
		_ = client.stdinPipeW.Close()
	}
	return client
}

func (c *pluginClient) startStdinForwarding() {
	c.stdinMu.Lock()
	if c.stdinForward {
		c.stdinMu.Unlock()
		return
	}
	c.stdinWriteMu.Lock()
	c.stdinForward = true
	pending := append([]byte(nil), c.pendingStdin.Bytes()...)
	closed := c.stdinClosed
	c.pendingStdin.Reset()
	c.stdinMu.Unlock()

	go func() {
		if len(pending) > 0 {
			_, _ = c.stdinPipeW.Write(pending)
		}
		if closed {
			_ = c.stdinPipeW.Close()
		}
		c.stdinWriteMu.Unlock()
	}()
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
	if req.Timeout > 0 {
		msg.Timeout = req.Timeout
	}
	reply, err := c.Do(&msg)
	if err != nil {
		return nil, err
	}
	return &HTTPResponse{
		Status:  reply.Status,
		Headers: reply.Headers,
		Body:    reply.Body,
		Error:   reply.Error,
	}, nil
}

func (c *pluginClient) fetchSpecSync(name string) (*APISpec, error) {
	reply, err := c.FetchAPISpec(name)
	if err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, fmt.Errorf("%s", reply.Error)
	}
	if len(reply.Operations) > 0 {
		return &APISpec{
			Name:        name,
			ContentType: reply.ContentType,
			Raw:         reply.Raw,
			Operations:  reply.Operations,
		}, nil
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
