package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
)

type terminalContext struct {
	Color bool
}

type pluginClient struct {
	in      io.Reader
	out     io.Writer
	term    terminalContext
	writeMu sync.Mutex
}

type httpResponse struct {
	Status  int
	Headers map[string]string
	Body    any
	Error   string
}

func newPluginClient(in io.Reader, out io.Writer, term terminalContext) *pluginClient {
	return &pluginClient{in: in, out: out, term: term}
}

func terminalContextFromArgs(args []string) terminalContext {
	var ctx terminalContext
	for _, arg := range args {
		if value, ok := strings.CutPrefix(arg, "--rsh-color="); ok {
			ctx.Color = value == "true"
		}
	}
	return ctx
}

func (c *pluginClient) request(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
	msg := map[string]any{
		"type":   "http-request",
		"method": method,
		"uri":    uri,
	}
	if len(headers) > 0 {
		msg["headers"] = headers
	}
	if body != nil {
		msg["body"] = body
		msg["content_type"] = "json"
	}
	if err := c.writeMessage(msg); err != nil {
		return nil, err
	}

	var reply map[string]any
	if err := pluginwire.ReadMessage(c.in, &reply); err != nil {
		return nil, err
	}
	if msgType, _ := reply["type"].(string); msgType != "http-response" {
		return nil, fmt.Errorf("unexpected plugin reply %q", msgType)
	}

	resp := &httpResponse{
		Status:  pluginwire.MsgInt(reply["status"]),
		Headers: mapString(reply["headers"]),
		Body:    reply["body"],
	}
	if text, _ := reply["error"].(string); text != "" {
		resp.Error = text
	}
	return resp, nil
}

func (c *pluginClient) stdout(data []byte) error {
	return c.writeMessage(map[string]any{"type": "stdout-data", "data": append([]byte(nil), data...)})
}

func (c *pluginClient) stderr(data []byte) error {
	return c.writeMessage(map[string]any{"type": "stderr-data", "data": append([]byte(nil), data...)})
}

func (c *pluginClient) warn(text string) error {
	return c.writeMessage(map[string]any{"type": "warn", "text": text})
}

func (c *pluginClient) response(resp *httpResponse) error {
	msg := map[string]any{
		"type":   "response",
		"status": resp.Status,
		"body":   resp.Body,
	}
	if len(resp.Headers) > 0 {
		msg["headers"] = resp.Headers
	}
	return c.writeMessage(msg)
}

func (c *pluginClient) done(code int) error {
	return c.writeMessage(map[string]any{"type": "done", "exit_code": code})
}

func (c *pluginClient) writeMessage(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return pluginwire.WriteMessage(c.out, v)
}

type streamWriter struct {
	write func([]byte) error
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := w.write(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func mapString(v any) map[string]string {
	items, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(items))
	for k, raw := range items {
		switch value := raw.(type) {
		case string:
			out[k] = value
		case []string:
			if len(value) > 0 {
				out[k] = value[0]
			}
		case []any:
			if len(value) > 0 {
				out[k] = fmt.Sprint(value[0])
			}
		default:
			if raw != nil {
				out[k] = fmt.Sprint(raw)
			}
		}
	}
	return out
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func parseBool(value string) bool {
	b, _ := strconv.ParseBool(value)
	return b
}
