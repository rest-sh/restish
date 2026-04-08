package main

import (
	"io"
	"strings"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
)

type terminalContext struct {
	Color bool
}

type pluginClient struct {
	*pluginwire.CommandClient
	term terminalContext
}

type httpResponse struct {
	Status  int
	Headers map[string]string
	Body    any
	Error   string
}

func newPluginClient(in io.Reader, out io.Writer, term terminalContext) *pluginClient {
	return &pluginClient{
		CommandClient: pluginwire.NewCommandClient(in, out),
		term:          term,
	}
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
	req := &pluginwire.HTTPRequestMsg{
		Method: method,
		URI:    uri,
	}
	if len(headers) > 0 {
		req.Headers = headers
	}
	if body != nil {
		req.Body = body
		req.ContentType = "json"
	}
	reply, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	resp := &httpResponse{
		Status:  reply.Status,
		Headers: reply.Headers,
		Body:    reply.Body,
		Error:   reply.Error,
	}
	return resp, nil
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
	return c.WriteMessage(msg)
}
