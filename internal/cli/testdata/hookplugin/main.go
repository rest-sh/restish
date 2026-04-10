// hookplugin is a test Restish plugin that implements auth, request-middleware,
// response-middleware, formatter, and loader hooks for unit tests.
//
// Auth hook:              adds Authorization: Bearer hook-token.
// Request middleware:     adds X-Trace-Id: hook-trace-123.
// Response middleware:    behaviour controlled by RSH_HOOK_RM_BEHAVIOR:
//
//	""          → adds "plugin_added":true to the response body (if map)
//	"drop"      → returns {"drop":true}
//	"follow:<u>"→ returns {"follow":{"method":"GET","uri":"<u>"}}
//
// Formatter hook:         writes "HOOK FORMATTED\n" to stdout (raw, no CBOR).
// Loader hook:            returns a minimal OpenAPI 3.0 JSON spec via CBOR.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/danielgtaylor/restish/v2/plugin"
)

// minimalOpenAPI is a minimal OpenAPI 3.0 spec returned by the loader hook.
const minimalOpenAPI = `{
  "openapi": "3.0.0",
  "info": {"title": "Hook Plugin API", "version": "1.0.0"},
  "paths": {
    "/hook-items": {
      "get": {
        "operationId": "listHookItems",
        "summary": "List hook items",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`

var manifest = plugin.Manifest{
	Name:               "hookplugin",
	Version:            "1.0.0",
	Description:        "Test hook plugin",
	RestishAPIVersion:  1,
	Hooks:              []string{"auth", "request-middleware", "response-middleware", "formatter", "loader"},
	FormatterNames:     []string{"hookformat"},
	LoaderContentTypes: []string{"application/x-hook-api"},
}

func main() {
	if plugin.HandleStartupFlags(os.Stdout, manifest, nil) {
		return
	}

	// Hook plugins receive exactly one message on stdin.
	raw, err := plugin.NewDecoder(os.Stdin).ReadRaw()
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	switch plugin.MessageType(raw) {
	case "auth":
		var msg plugin.AuthHookInput
		_ = plugin.DecMode.Unmarshal(raw, &msg)
		out := plugin.AuthHookOutput{
			Request: &plugin.HookRequestHeaderUpdate{
				Headers: map[string]any{
					"Authorization": []any{"Bearer hook-token"},
				},
			},
		}
		if err := plugin.WriteMessage(os.Stdout, out); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}

	case "request-middleware":
		var msg plugin.RequestMiddlewareInput
		_ = plugin.DecMode.Unmarshal(raw, &msg)
		hdrs := map[string]any{}
		for k, vs := range msg.Request.Headers {
			hdrs[k] = vs
		}
		hdrs["X-Trace-Id"] = []any{"hook-trace-123"}
		out := plugin.RequestMiddlewareOutput{
			Request: &plugin.HookRequestHeaderUpdate{Headers: hdrs},
		}
		if err := plugin.WriteMessage(os.Stdout, out); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}

	case "response-middleware":
		var msg plugin.ResponseMiddlewareInput
		_ = plugin.DecMode.Unmarshal(raw, &msg)

		behavior := os.Getenv("RSH_HOOK_RM_BEHAVIOR")
		switch {
		case behavior == "drop":
			plugin.WriteMessage(os.Stdout, plugin.ResponseMiddlewareOutput{Drop: true}) //nolint:errcheck
		case strings.HasPrefix(behavior, "follow:"):
			uri := strings.TrimPrefix(behavior, "follow:")
			plugin.WriteMessage(os.Stdout, plugin.ResponseMiddlewareOutput{ //nolint:errcheck
				Follow: &plugin.FollowRequest{Method: "GET", URI: uri},
			})
		default:
			body, _ := msg.Response.Body.(map[string]any)
			if body == nil {
				body = map[string]any{}
			}
			body["plugin_added"] = true
			plugin.WriteMessage(os.Stdout, plugin.ResponseMiddlewareOutput{ //nolint:errcheck
				Response: &plugin.HookResponseUpdate{Body: body},
			})
		}

	case "formatter":
		fmt.Fprint(os.Stdout, "HOOK FORMATTED\n")

	case "loader":
		out := map[string]any{
			"content_type": "application/openapi+json",
			"body":         minimalOpenAPI,
		}
		if err := plugin.WriteMessage(os.Stdout, out); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}

	default:
		os.Exit(0)
	}
}
