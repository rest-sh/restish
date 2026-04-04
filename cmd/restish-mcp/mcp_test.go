package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
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

	tools, err := toolsFromSpec("demo", true, s, Options{})
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
