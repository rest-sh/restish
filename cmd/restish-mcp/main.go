package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
	"github.com/pb33f/libopenapi"
)

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--rsh-plugin-manifest":
			writeCBOR(Manifest())
			return
		case "--rsh-plugin-commands":
			writeCBOR(Commands())
			return
		}
	}

	var initMsg map[string]any
	if err := plugin.ReadMessage(os.Stdin, &initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}
	command, _ := initMsg["command"].(string)
	if command != "mcp" {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 1})
		return
	}

	args := msgStrings(initMsg["args"])
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
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "stderr-data", "data": []byte(err.Error() + "\n")})
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 1})
		return
	}
	_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 0})
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
		case "stdin-data":
			if data := msgBytes(msg["data"]); len(data) > 0 {
				if _, err := c.stdinPipeW.Write(data); err != nil {
					return
				}
			}
		case "stdin-close":
			_ = c.stdinPipeW.Close()
		case "http-response":
			c.httpRespCh <- msg
		}
	}
}

func (c *pluginClient) do(req *HTTPRequest) (*HTTPResponse, error) {
	msg := map[string]any{
		"type":   "http-request",
		"method": req.Method,
		"uri":    req.URI,
	}
	if len(req.Headers) > 0 {
		msg["headers"] = req.Headers
	}
	if req.Body != nil {
		msg["body"] = req.Body
	}
	if req.ContentType != "" {
		msg["content_type"] = req.ContentType
	}
	if err := c.writeMessage(msg); err != nil {
		return nil, err
	}
	reply, ok := <-c.httpRespCh
	if !ok {
		return nil, io.EOF
	}
	resp := &HTTPResponse{
		Status:  msgInt(reply["status"]),
		Headers: mapAny(reply["headers"]),
		Body:    reply["body"],
	}
	if text, _ := reply["error"].(string); text != "" {
		resp.Error = text
	}
	return resp, nil
}

func (c *pluginClient) fetchSpecSync(name string) (*APISpec, error) {
	if err := c.writeMessage(map[string]any{
		"type": "api-spec",
		"name": name,
	}); err != nil {
		return nil, err
	}
	var reply map[string]any
	for {
		if err := plugin.ReadMessage(c.in, &reply); err != nil {
			return nil, err
		}
		switch msgType, _ := reply["type"].(string); msgType {
		case "api-spec-response":
			goto haveReply
		case "stdin-data":
			if data := msgBytes(reply["data"]); len(data) > 0 {
				_, _ = c.pendingStdin.Write(data)
			}
		case "stdin-close":
			c.stdinClosed = true
		default:
			return nil, fmt.Errorf("unexpected plugin reply %q while loading %s", msgType, name)
		}
	}
haveReply:
	if text, _ := reply["error"].(string); text != "" {
		return nil, fmt.Errorf("%s", text)
	}
	raw := msgBytes(reply["raw"])
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
	if err := w.client.writeMessage(map[string]any{"type": "stdout-data", "data": append([]byte(nil), p...)}); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *pluginClient) newStdoutWriter() io.Writer {
	return &stdoutWriter{client: c}
}

func writeCBOR(v any) {
	data, err := cbor.Marshal(v)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(2)
	}
	_, _ = os.Stdout.Write(data)
}

func msgStrings(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func msgBytes(v any) []byte {
	switch data := v.(type) {
	case []byte:
		return data
	case string:
		return []byte(data)
	case []any:
		out := make([]byte, 0, len(data))
		for _, item := range data {
			switch n := item.(type) {
			case uint64:
				out = append(out, byte(n))
			case int64:
				out = append(out, byte(n))
			case int:
				out = append(out, byte(n))
			}
		}
		return out
	default:
		return nil
	}
}

func msgInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func mapAny(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}
