package main

import (
	"fmt"
	"io"
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
	msg := pluginwire.HTTPRequestMsg{
		Type:   pluginwire.MsgTypeHTTPRequest,
		Method: method,
		URI:    uri,
	}
	if len(headers) > 0 {
		msg.Headers = headers
	}
	if body != nil {
		msg.Body = body
		msg.ContentType = "json"
	}
	if err := c.writeMessage(msg); err != nil {
		return nil, err
	}

	var reply pluginwire.HTTPResponseMsg
	if err := pluginwire.ReadMessage(c.in, &reply); err != nil {
		return nil, err
	}
	if reply.Type != pluginwire.MsgTypeHTTPResponse {
		return nil, fmt.Errorf("unexpected plugin reply %q", reply.Type)
	}

	resp := &httpResponse{
		Status:  reply.Status,
		Headers: reply.Headers,
		Body:    reply.Body,
	}
	if reply.Error != "" {
		resp.Error = reply.Error
	}
	return resp, nil
}

func (c *pluginClient) stdout(data []byte) error {
	return c.writeMessage(pluginwire.StdoutDataMsg{Type: pluginwire.MsgTypeStdoutData, Data: append([]byte(nil), data...)})
}

func (c *pluginClient) stderr(data []byte) error {
	return c.writeMessage(pluginwire.StderrDataMsg{Type: pluginwire.MsgTypeStderrData, Data: append([]byte(nil), data...)})
}

func (c *pluginClient) warn(text string) error {
	return c.writeMessage(pluginwire.WarnMsg{Type: pluginwire.MsgTypeWarn, Text: text})
}

func (c *pluginClient) response(resp *httpResponse) error {
	msg := pluginwire.ResponseMsg{
		Type:   pluginwire.MsgTypeResponse,
		Status: resp.Status,
		Body:   resp.Body,
	}
	if len(resp.Headers) > 0 {
		msg.Headers = resp.Headers
	}
	return c.writeMessage(msg)
}

func (c *pluginClient) done(code int) error {
	return c.writeMessage(pluginwire.DoneMsg{Type: pluginwire.MsgTypeDone, ExitCode: code})
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
