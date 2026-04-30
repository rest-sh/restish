package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	"github.com/rest-sh/restish/v2/plugin"
)

func loadTestSpec(t *testing.T, name, raw string) *APISpec {
	t.Helper()
	doc, err := libopenapi.NewDocument([]byte(raw))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	return &APISpec{Name: name, ContentType: "application/json", Raw: []byte(raw), Document: doc}
}

func TestToolsFromSpecFilteringAndNamespacing(t *testing.T) {
	s := loadTestSpec(t, "demo", `{
	  "openapi": "3.1.0",
	  "info": {"title": "Demo", "version": "1.0.0"},
	  "paths": {
	    "/items/{id}": {
	      "get": {
	        "operationId": "getItem",
	        "summary": "Get an item",
	        "parameters": [
	          {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
	        ]
	      }
	    },
	    "/items": {
	      "post": {
	        "operationId": "createItem",
	        "requestBody": {
	          "required": true,
	          "content": {
	            "application/json": {
	              "schema": {
	                "type": "object",
	                "properties": {"name": {"type": "string"}}
	              }
	            }
	          }
	        }
	      }
	    },
	    "/hidden": {
	      "get": {
	        "operationId": "hiddenCli",
	        "x-cli-ignore": true
	      }
	    },
	    "/mcp-hidden": {
	      "get": {
	        "operationId": "hiddenMcp",
	        "x-mcp-ignore": true
	      }
	    }
	  }
	}`)

	tools, err := toolsFromSpec("demo", true, s, Options{AllowWriteTools: true})
	if err != nil {
		t.Fatalf("toolsFromSpec: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 visible tools, got %d", len(tools))
	}

	var create *Tool
	for _, tool := range tools {
		if tool.Name == "demo__createItem" {
			create = tool
		}
	}
	if create == nil {
		t.Fatal("expected namespaced createItem tool")
	}
	if create.BodyContentType != "application/json" {
		t.Fatalf("expected json body content type, got %q", create.BodyContentType)
	}
	props := create.InputSchema["properties"].(map[string]any)
	if _, ok := props["body"]; !ok {
		t.Fatal("expected body property in input schema")
	}
}

func TestToolsFromSpecSkipsWriteOperationsByDefault(t *testing.T) {
	s := loadTestSpec(t, "demo", `{
	  "openapi": "3.1.0",
	  "info": {"title": "Demo", "version": "1.0.0"},
	  "paths": {
	    "/items": {
	      "get": {"operationId": "listItems"},
	      "post": {"operationId": "createItem"},
	      "put": {"operationId": "replaceItem"},
	      "patch": {"operationId": "patchItem"},
	      "delete": {"operationId": "deleteItem"}
	    },
	    "/options": {
	      "options": {"operationId": "optionsItems"}
	    }
	  }
	}`)

	tools, err := toolsFromSpec("demo", false, s, Options{})
	if err != nil {
		t.Fatalf("toolsFromSpec: %v", err)
	}
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "listItems,optionsItems" {
		t.Fatalf("tools = %v, want listItems and optionsItems", names)
	}
}

func TestToolsFromSpecAllowWriteToolsExposesWrites(t *testing.T) {
	s := &APISpec{
		Name: "demo",
		Operations: []plugin.APIOperation{
			{ID: "getItem", Method: "GET", Path: "/items"},
			{ID: "createItem", Method: "POST", Path: "/items"},
			{ID: "replaceItem", Method: "PUT", Path: "/items"},
			{ID: "patchItem", Method: "PATCH", Path: "/items"},
			{ID: "deleteItem", Method: "DELETE", Path: "/items"},
		},
	}

	tools, err := toolsFromSpec("demo", false, s, Options{AllowWriteTools: true})
	if err != nil {
		t.Fatalf("toolsFromSpec: %v", err)
	}
	if len(tools) != 5 {
		t.Fatalf("expected all write tools with --allow-write-tools, got %#v", tools)
	}
}

func TestToolsFromSpecIncludesPathLevelParameters(t *testing.T) {
	s := loadTestSpec(t, "demo", `{
	  "openapi": "3.1.0",
	  "info": {"title": "Demo", "version": "1.0.0"},
	  "paths": {
	    "/items/{id}": {
	      "parameters": [
	        {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
	        {"name": "verbose", "in": "query", "schema": {"type": "boolean"}}
	      ],
	      "get": {
	        "operationId": "getItem",
	        "parameters": [
	          {"name": "verbose", "in": "query", "schema": {"type": "string"}}
	        ]
	      }
	    }
	  }
	}`)

	tools, err := toolsFromSpec("demo", false, s, Options{})
	if err != nil {
		t.Fatalf("toolsFromSpec: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0]
	props := tool.InputSchema["properties"].(map[string]any)
	if _, ok := props["id"]; !ok {
		t.Fatalf("expected path-level id parameter in schema: %#v", props)
	}
	required := tool.InputSchema["required"].([]string)
	if len(required) != 1 || required[0] != "id" {
		t.Fatalf("required = %#v, want [id]", required)
	}
	if got := props["verbose"].(map[string]any)["type"]; got != "string" {
		t.Fatalf("operation-level verbose parameter should override path-level schema, got %#v", got)
	}

	req, err := tool.Request(map[string]any{"id": "a/b", "verbose": "yes"})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if req.URI != "demo/items/a%2Fb?verbose=yes" {
		t.Fatalf("URI = %q, want demo/items/a%%2Fb?verbose=yes", req.URI)
	}
}

func TestToolsFromSpecPrefersHostResolvedOperations(t *testing.T) {
	s := &APISpec{
		Name: "demo",
		Operations: []plugin.APIOperation{
			{
				ID:      "getItem",
				Method:  "GET",
				Path:    "/v2/items/{id}",
				Summary: "Get an item",
				Parameters: []plugin.APIParam{
					{Name: "id", In: "path", Required: true, Type: "string"},
					{Name: "include", In: "query", Type: "boolean"},
				},
			},
			{ID: "hiddenMcp", Method: "GET", Path: "/hidden", MCPIgnore: true},
			{ID: "createItem", Method: "POST", Path: "/v2/items"},
		},
	}

	tools, err := toolsFromSpec("demo", false, s, Options{})
	if err != nil {
		t.Fatalf("toolsFromSpec: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "getItem" {
		t.Fatalf("expected only getItem, got %#v", tools)
	}
	req, err := tools[0].Request(map[string]any{"id": "a/b", "include": true})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if req.URI != "demo/v2/items/a%2Fb?include=true" {
		t.Fatalf("URI = %q, want demo/v2/items/a%%2Fb?include=true", req.URI)
	}
}

func TestParseArgsRejectsRemovedHTTPFlag(t *testing.T) {
	if _, err := ParseArgs([]string{"--http", ":3000", "demo"}); err == nil {
		t.Fatal("expected removed --http flag to be rejected by flag parser")
	}
}

func TestParseArgsRequestTimeout(t *testing.T) {
	cfg, err := ParseArgs([]string{"--request-timeout", "7", "demo"})
	if err != nil {
		t.Fatalf("ParseArgs: %v", err)
	}
	if got := cfg.Options.RequestTimeout; got != 7 {
		t.Fatalf("RequestTimeout = %d, want 7", got)
	}
}

func TestPluginClientSendsHTTPRequestTimeout(t *testing.T) {
	var out bytes.Buffer
	client := newPluginClient(plugin.NewDecoder(bytes.NewReader(nil)), &out)
	client.httpRespCh <- plugin.HTTPResponseMsg{Type: plugin.MsgTypeHTTPResponse, RequestID: "1", Status: 200}

	if _, err := client.do(&HTTPRequest{Method: "GET", URI: "demo/items", Timeout: 9}); err != nil {
		t.Fatalf("do: %v", err)
	}

	var msg plugin.HTTPRequestMsg
	if err := plugin.ReadMessage(&out, &msg); err != nil {
		t.Fatalf("decode request message: %v", err)
	}
	if got := msg.Timeout; got != 9 {
		t.Fatalf("Timeout = %d, want 9", got)
	}
}

func TestRunServeToolCall(t *testing.T) {
	spec := loadTestSpec(t, "demo", `{
	  "openapi": "3.1.0",
	  "info": {"title": "Demo", "version": "1.0.0"},
	  "paths": {
	    "/items/{id}": {
	      "get": {
	        "operationId": "getItem",
	        "summary": "Get an item",
	        "parameters": [
	          {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}
	        ]
	      }
	    }
	  }
	}`)

	var stdin bytes.Buffer
	writeFrame(&stdin, mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{},
	}))
	writeFrame(&stdin, mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	}))
	writeFrame(&stdin, mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "getItem",
			"arguments": map[string]any{"id": "42"},
		},
	}))

	var stdout bytes.Buffer
	err := Run(&stdin, &stdout, func(name string) (*APISpec, error) {
		if name != "demo" {
			t.Fatalf("unexpected api name: %s", name)
		}
		return spec, nil
	}, func(req *HTTPRequest) (*HTTPResponse, error) {
		if req.URI != "demo/items/42" {
			t.Fatalf("unexpected request URI: %s", req.URI)
		}
		if req.Timeout != 60 {
			t.Fatalf("request timeout = %d, want default 60", req.Timeout)
		}
		return &HTTPResponse{
			Status: 200,
			Body:   map[string]any{"id": "42", "name": "example"},
		}, nil
	}, []string{"demo"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	responses := readResponses(t, stdout.Bytes())
	if len(responses) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(responses))
	}
	listResult := responses[1]["result"].(map[string]any)
	tools := listResult["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"].(string) != "getItem" {
		t.Fatalf("unexpected tools list: %#v", tools)
	}
	last := responses[2]["result"].(map[string]any)
	content := last["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, `"name": "example"`) {
		t.Fatalf("expected tool body in result, got:\n%s", text)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

func readResponses(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	reader := bufio.NewReader(bytes.NewReader(data))
	for {
		payload, err := readFrame(reader)
		if err != nil {
			break
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		out = append(out, msg)
	}
	return out
}
