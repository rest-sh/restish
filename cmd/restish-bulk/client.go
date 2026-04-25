package main

import pluginwire "github.com/rest-sh/restish/v2/plugin"

type pluginClient struct {
	*pluginwire.CommandClient
	term        pluginwire.TerminalContext
	requestFunc func(method, uri string, headers map[string]string, body any) (*httpResponse, error)
}

type httpResponse struct {
	Status  int
	Headers map[string]string
	Body    any
	Error   string
}

func (c *pluginClient) request(method, uri string, headers map[string]string, body any) (*httpResponse, error) {
	if c.requestFunc != nil {
		return c.requestFunc(method, uri, headers, body)
	}
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
