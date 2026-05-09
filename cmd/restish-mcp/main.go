package main

import (
	"errors"
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
	if text, ok := helpText(args); ok {
		_ = plugin.WriteMessage(os.Stdout, plugin.StdoutDataMsg{Type: plugin.MsgTypeStdoutData, Data: []byte(text)})
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone})
		return
	}
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

func helpText(args []string) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	switch args[0] {
	case "-h", "--help", "help":
		return rootHelpText(), true
	case "serve":
		for _, arg := range args[1:] {
			if arg == "-h" || arg == "--help" {
				return serveHelpText(), true
			}
		}
	}
	return "", false
}

func rootHelpText() string {
	return "Expose registered APIs as MCP tools via Restish-authenticated HTTP delegation.\n\nUsage:\n  restish mcp serve <api...>\n\nCommands:\n  serve    Serve registered APIs over stdio\n"
}

func serveHelpText() string {
	return "Serve registered APIs over the Model Context Protocol.\n\nUsage:\n  restish mcp serve [flags] <api...>\n\nFlags:\n  --operations string        Comma-separated operationId allowlist\n  --max-result-bytes int     Maximum tool result payload size\n  --request-timeout int      Per-tool HTTP request timeout in seconds (0 disables)\n  --read-only                Expose only GET/HEAD operations\n  --allow-write-tools        Expose POST, PUT, PATCH, and DELETE operations as MCP tools\n"
}

const stdinForwardQueueSize = 64

var errStdinForwardQueueFull = errors.New("mcp: stdin passthrough queue full; MCP client is not reading stdin fast enough")

type stdinForwardEvent struct {
	data  []byte
	close bool
}

type pluginClient struct {
	*plugin.CommandClient
	stdinPipeW   *io.PipeWriter
	stdinReader  io.Reader
	stdinWriteMu sync.Mutex
	stdinClose   sync.Once
	stdinFailure sync.Once
	stdinQueue   chan stdinForwardEvent
}

func newPluginClient(dec *plugin.Decoder, out io.Writer) *pluginClient {
	pr, pw := io.Pipe()
	client := &pluginClient{
		CommandClient: plugin.NewCommandClientFromDecoder(dec, out),
		stdinPipeW:    pw,
		stdinReader:   pr,
		stdinQueue:    make(chan stdinForwardEvent, stdinForwardQueueSize),
	}
	go client.forwardStdin()
	client.StdinDataHandler = func(data []byte) {
		client.enqueueStdin(stdinForwardEvent{data: append([]byte(nil), data...)})
	}
	client.StdinCloseHandler = func() {
		client.enqueueStdin(stdinForwardEvent{close: true})
	}
	return client
}

func (c *pluginClient) enqueueStdin(event stdinForwardEvent) {
	select {
	case c.stdinQueue <- event:
	default:
		c.failStdinForwarding(errStdinForwardQueueFull)
	}
}

func (c *pluginClient) forwardStdin() {
	for event := range c.stdinQueue {
		if event.close {
			c.closeStdin(nil)
			return
		}
		if len(event.data) == 0 {
			continue
		}
		c.stdinWriteMu.Lock()
		_, err := c.stdinPipeW.Write(event.data)
		c.stdinWriteMu.Unlock()
		if err != nil {
			return
		}
	}
}

func (c *pluginClient) failStdinForwarding(err error) {
	c.stdinFailure.Do(func() {
		_ = c.WriteStderr([]byte(err.Error() + "\n"))
		c.closeStdin(err)
	})
}

func (c *pluginClient) closeStdin(err error) {
	c.stdinClose.Do(func() {
		if err != nil {
			_ = c.stdinPipeW.CloseWithError(err)
			return
		}
		_ = c.stdinPipeW.Close()
	})
}

func (c *pluginClient) startStdinForwarding() {
	// Compatibility no-op for tests and older call sites. Forwarding starts when
	// the plugin client is created so pre-startup MCP frames are bounded.
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
