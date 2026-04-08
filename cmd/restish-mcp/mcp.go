package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pb33f/libopenapi"
	v3high "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/danielgtaylor/restish/v2/internal/spec"
)

const DefaultMaxResultBytes = 16 * 1024

// maxRPCPayloadBytes caps the Content-Length accepted from an MCP client to
// prevent memory exhaustion. Matches the CBOR plugin protocol limit.
const maxRPCPayloadBytes = 64 << 20 // 64 MiB

type HTTPRequest struct {
	Method      string
	URI         string
	Headers     map[string]string
	Body        any
	ContentType string
}

type HTTPResponse struct {
	Status  int
	Headers map[string]any
	Body    any
	Error   string
}

type APISpec struct {
	Name        string
	ContentType string
	Raw         []byte
	Document    libopenapi.Document
}

type HTTPExecutor func(*HTTPRequest) (*HTTPResponse, error)
type SpecFetcher func(name string) (*APISpec, error)

type Tool struct {
	APIName         string
	Name            string
	Description     string
	Method          string
	Path            string
	InputSchema     map[string]any
	Params          []Param
	BodyContentType string
	BodyRequired    bool
}

type Param struct {
	Name        string
	In          string
	Required    bool
	Description string
}

type Options struct {
	Operations     map[string]bool
	ReadOnly       bool
	MaxResultBytes int
}

type Server struct {
	Tools          []*Tool
	ToolIndex      map[string]*Tool
	Exec           HTTPExecutor
	MaxResultBytes int
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Manifest() map[string]any {
	return map[string]any{
		"name":                "mcp",
		"version":             "1.0.0",
		"description":         "Expose registered APIs as MCP tools",
		"restish_api_version": 1,
		"hooks":               []string{"command"},
	}
}

func Commands() map[string]any {
	return map[string]any{
		"commands": []any{
			map[string]any{
				"name":              "mcp",
				"short":             "Serve registered APIs over the Model Context Protocol",
				"long":              "Expose OpenAPI operations as MCP tools via Restish-authenticated HTTP delegation.",
				"passthrough_stdio": true,
			},
		},
	}
}

func Run(stdin io.Reader, stdout io.Writer, fetchSpec SpecFetcher, exec HTTPExecutor, args []string) error {
	cfg, err := ParseArgs(args)
	if err != nil {
		return err
	}
	tools, err := LoadTools(fetchSpec, cfg.APINames, cfg.Options)
	if err != nil {
		return err
	}
	server := &Server{
		Tools:          tools,
		ToolIndex:      indexTools(tools),
		Exec:           exec,
		MaxResultBytes: cfg.Options.MaxResultBytes,
	}
	if server.MaxResultBytes <= 0 {
		server.MaxResultBytes = DefaultMaxResultBytes
	}
	return server.ServeStdio(stdin, stdout)
}

type ServeConfig struct {
	APINames []string
	Options  Options
}

func ParseArgs(args []string) (*ServeConfig, error) {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var operations string
	var httpAddr string
	var maxResultBytes int
	var readOnly bool
	fs.StringVar(&operations, "operations", "", "Comma-separated operationId allowlist")
	fs.StringVar(&httpAddr, "http", "", "HTTP transport address (not yet implemented)")
	fs.IntVar(&maxResultBytes, "max-result-bytes", DefaultMaxResultBytes, "Maximum tool result payload size")
	fs.BoolVar(&readOnly, "read-only", false, "Expose only GET/HEAD operations")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if httpAddr != "" {
		return nil, errors.New("--http is not implemented yet")
	}
	apiNames := fs.Args()
	if len(apiNames) == 0 {
		return nil, errors.New("mcp requires at least one API name")
	}
	ops := map[string]bool{}
	for _, item := range strings.Split(operations, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			ops[item] = true
		}
	}
	return &ServeConfig{
		APINames: apiNames,
		Options: Options{
			Operations:     ops,
			ReadOnly:       readOnly,
			MaxResultBytes: maxResultBytes,
		},
	}, nil
}

func LoadTools(fetchSpec SpecFetcher, apiNames []string, opts Options) ([]*Tool, error) {
	multiAPI := len(apiNames) > 1
	var tools []*Tool
	for _, apiName := range apiNames {
		s, err := fetchSpec(apiName)
		if err != nil {
			return nil, err
		}
		apiTools, err := toolsFromSpec(apiName, multiAPI, s, opts)
		if err != nil {
			return nil, err
		}
		tools = append(tools, apiTools...)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools, nil
}

func toolsFromSpec(apiName string, multiAPI bool, s *APISpec, opts Options) ([]*Tool, error) {
	model, err := s.Document.BuildV3Model()
	if err != nil || model == nil || model.Model.Paths == nil {
		return nil, fmt.Errorf("building OpenAPI model for %q: %w", apiName, err)
	}

	var tools []*Tool
	for path, pathItem := range model.Model.Paths.PathItems.FromOldest() {
		for _, item := range spec.PathItemMethods(pathItem) {
			if item.Op == nil || item.Op.OperationId == "" {
				continue
			}
			if spec.OpExtBool(item.Op, "x-cli-ignore") || spec.OpExtBool(item.Op, "x-mcp-ignore") {
				continue
			}
			if opts.ReadOnly && item.Method != "GET" && item.Method != "HEAD" {
				continue
			}
			if len(opts.Operations) > 0 && !opts.Operations[item.Op.OperationId] {
				continue
			}
			tool, err := buildTool(apiName, multiAPI, path, item.Method, item.Op)
			if err != nil {
				return nil, err
			}
			tools = append(tools, tool)
		}
	}
	return tools, nil
}

func buildTool(apiName string, multiAPI bool, path, method string, op *v3high.Operation) (*Tool, error) {
	name := op.OperationId
	if multiAPI {
		name = apiName + "__" + name
	}
	description := strings.TrimSpace(op.Summary)
	if description == "" {
		description = strings.TrimSpace(op.Description)
	}
	if description == "" {
		description = method + " " + path
	}

	properties := map[string]any{}
	var required []string
	var params []Param
	for _, p := range op.Parameters {
		params = append(params, Param{
			Name:        p.Name,
			In:          p.In,
			Required:    p.Required != nil && *p.Required,
			Description: p.Description,
		})
		prop := schemaMap(nil)
		if p.Schema != nil && p.Schema.Schema() != nil {
			prop = schemaMap(p.Schema.Schema())
		}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		properties[p.Name] = prop
		if p.Required != nil && *p.Required {
			required = append(required, p.Name)
		}
	}

	bodyContentType, bodyRequired := "", false
	if op.RequestBody != nil {
		bodyContentType, properties["body"] = requestBodyProperty(op.RequestBody)
		bodyRequired = requestBodyRequired(op.RequestBody)
		if bodyRequired {
			required = append(required, "body")
		}
	}

	inputSchema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}
	return &Tool{
		APIName:         apiName,
		Name:            name,
		Description:     description,
		Method:          method,
		Path:            path,
		InputSchema:     inputSchema,
		Params:          params,
		BodyContentType: bodyContentType,
		BodyRequired:    bodyRequired,
	}, nil
}

func requestBodyProperty(body *v3high.RequestBody) (string, map[string]any) {
	if body == nil || body.Content == nil {
		return "", map[string]any{"type": "object"}
	}
	for _, contentType := range []string{"application/json", "application/merge-patch+json"} {
		if media := body.Content.GetOrZero(contentType); media != nil {
			return contentType, bodySchema(media, body.Description)
		}
	}
	for contentType, media := range body.Content.FromOldest() {
		if media != nil {
			return contentType, bodySchema(media, body.Description)
		}
	}
	return "", map[string]any{"type": "object"}
}

func requestBodyRequired(body *v3high.RequestBody) bool {
	return body != nil && body.Required != nil && *body.Required
}

func bodySchema(media *v3high.MediaType, description string) map[string]any {
	prop := map[string]any{"type": "object"}
	if media != nil && media.Schema != nil && media.Schema.Schema() != nil {
		prop = schemaMap(media.Schema.Schema())
	}
	if description != "" {
		prop["description"] = description
	}
	return prop
}

func schemaMap(v any) map[string]any {
	if v == nil {
		return map[string]any{"type": "string"}
	}
	data, err := json.Marshal(v)
	if err != nil || len(data) == 0 {
		return map[string]any{"type": "string"}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil || len(out) == 0 {
		return map[string]any{"type": "string"}
	}
	return out
}

func indexTools(tools []*Tool) map[string]*Tool {
	out := make(map[string]*Tool, len(tools))
	for _, tool := range tools {
		out[tool.Name] = tool
	}
	return out
}

func (s *Server) ServeStdio(stdin io.Reader, stdout io.Writer) error {
	reader := bufio.NewReader(stdin)
	for {
		payload, err := readFrame(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			if writeErr := writeRPC(stdout, rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			}); writeErr != nil {
				return writeErr
			}
			continue
		}
		if req.Method == "" {
			continue
		}
		resp := s.handleRequest(req)
		if req.ID == nil || resp.JSONRPC == "" {
			continue
		}
		if err := writeRPC(stdout, resp); err != nil {
			return err
		}
	}
}

func (s *Server) handleRequest(req rpcRequest) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2025-03-26",
			"serverInfo": map[string]any{
				"name":    "restish-mcp",
				"version": "1.0.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
		}
	case "notifications/initialized":
		return rpcResponse{}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.Tools))
		for _, tool := range s.Tools {
			tools = append(tools, map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"inputSchema": tool.InputSchema,
			})
		}
		resp.Result = map[string]any{"tools": tools}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: "invalid params"}
			return resp
		}
		tool := s.ToolIndex[params.Name]
		if tool == nil {
			resp.Error = &rpcError{Code: -32601, Message: "unknown tool"}
			return resp
		}
		result, err := s.callTool(tool, params.Arguments)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = result
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return resp
}

func (s *Server) callTool(tool *Tool, args map[string]any) (map[string]any, error) {
	if args == nil {
		args = map[string]any{}
	}
	req, err := tool.Request(args)
	if err != nil {
		return nil, err
	}
	if s.Exec == nil {
		return nil, errors.New("no HTTP executor configured")
	}
	resp, err := s.Exec(req)
	if err != nil {
		return nil, err
	}
	text, isError := formatToolResult(resp, s.MaxResultBytes)
	return map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
		"isError": isError,
	}, nil
}

func (t *Tool) Request(args map[string]any) (*HTTPRequest, error) {
	path := t.Path
	query := url.Values{}
	headers := map[string]string{}
	var cookies []string

	for _, param := range t.Params {
		value, ok := args[param.Name]
		if !ok || value == nil {
			if param.Required {
				return nil, fmt.Errorf("missing required parameter %q", param.Name)
			}
			continue
		}
		text := valueString(value)
		switch param.In {
		case "path":
			path = strings.ReplaceAll(path, "{"+param.Name+"}", url.PathEscape(text))
		case "query":
			query.Set(param.Name, text)
		case "header":
			headers[param.Name] = text
		case "cookie":
			cookies = append(cookies, param.Name+"="+url.QueryEscape(text))
		}
	}
	if len(cookies) > 0 {
		headers["Cookie"] = strings.Join(cookies, "; ")
	}
	rawURL := t.APIName + path
	if qs := query.Encode(); qs != "" {
		rawURL += "?" + qs
	}

	var body any
	if value, ok := args["body"]; ok {
		body = value
	} else if t.BodyRequired {
		return nil, errors.New("missing required parameter \"body\"")
	}

	return &HTTPRequest{
		Method:      t.Method,
		URI:         rawURL,
		Headers:     headers,
		Body:        body,
		ContentType: t.BodyContentType,
	}, nil
}

func formatToolResult(resp *HTTPResponse, maxBytes int) (string, bool) {
	isError := resp == nil || resp.Error != "" || resp.Status >= 400
	if resp == nil {
		return "request failed", true
	}

	bodyText := marshalPretty(resp.Body)
	if bodyText == "" {
		bodyText = "null"
	}
	if resp.Error != "" {
		bodyText = resp.Error
	}

	if resp.Status >= 400 {
		bodyText = fmt.Sprintf("HTTP %d\n%s", resp.Status, bodyText)
	} else if includeEnvelope(resp) {
		bodyText = marshalPretty(map[string]any{
			"status":  resp.Status,
			"headers": resp.Headers,
			"body":    resp.Body,
		})
	}

	if maxBytes > 0 && len(bodyText) > maxBytes {
		// Walk back to a UTF-8 code-point boundary so we don't split a multi-byte rune.
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(bodyText[cut]) {
			cut--
		}
		bodyText = bodyText[:cut] + "\n... truncated ..."
	}
	return bodyText, isError
}

func includeEnvelope(resp *HTTPResponse) bool {
	if resp == nil {
		return false
	}
	if resp.Status == 201 && resp.Headers != nil {
		if _, ok := resp.Headers["Location"]; ok {
			return true
		}
		if _, ok := resp.Headers["location"]; ok {
			return true
		}
	}
	return false
}

func marshalPretty(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		return string(data)
	}
	return fmt.Sprint(v)
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			value := strings.TrimSpace(line[len("content-length:"):])
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid content-length %q", value)
			}
			if n < 0 || n > maxRPCPayloadBytes {
				return nil, fmt.Errorf("content-length %d out of range (max %d)", n, maxRPCPayloadBytes)
			}
			length = n
		}
	}
	if length < 0 {
		return nil, errors.New("missing Content-Length header")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeRPC(w io.Writer, resp rpcResponse) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return writeFrame(w, data)
}

func writeFrame(w io.Writer, payload []byte) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(payload))
	buf.Write(payload)
	_, err := w.Write(buf.Bytes())
	return err
}

func valueString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}
