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

	"github.com/rest-sh/restish/v2/plugin"
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
	RestishAPIVersion:  2,
	Hooks:              []string{"auth", "request-middleware", "response-middleware", "formatter", "loader"},
	FormatterNames:     []string{"hookformat"},
	LoaderContentTypes: []string{"application/x-hook-api"},
}

func main() {
	if os.Getenv("RSH_HOOK_NEEDS_AUTH_SECRETS") == "1" {
		manifest.NeedsAuthSecrets = true
	}
	if plugin.HandleStartupFlags(os.Stdout, manifest, nil) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)
	raw, err := dec.ReadRaw()
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	switch plugin.MessageType(raw) {
	case "auth":
		var msg plugin.AuthHookInput
		_ = plugin.DecMode.Unmarshal(raw, &msg)
		checkSecretHeaders(msg.Type, msg.Request.Headers)
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
		checkSecretHeaders(msg.Type, msg.Request.Headers)
		out := plugin.RequestMiddlewareOutput{
			Request: &plugin.HookRequestHeaderUpdate{
				Headers: map[string]any{"X-Trace-Id": []any{"hook-trace-123"}},
			},
		}
		if err := plugin.WriteMessage(os.Stdout, out); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}

	case "response-middleware":
		var msg plugin.ResponseMiddlewareInput
		_ = plugin.DecMode.Unmarshal(raw, &msg)
		checkSecretHeaders(msg.Type, msg.Request.Headers)

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
		var first plugin.FormatterRequest
		_ = plugin.DecMode.Unmarshal(raw, &first)
		for {
			if first.Event == "start" {
				fmt.Fprint(os.Stdout, "HOOK FORMATTED\n")
			}
			if first.Event == "end" {
				break
			}
			if err := dec.ReadMessage(&first); err != nil {
				fmt.Fprintln(os.Stderr, "read:", err)
				os.Exit(1)
			}
		}

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

func checkSecretHeaders(hook string, headers map[string][]string) {
	expect := os.Getenv("RSH_HOOK_EXPECT_SECRET_HEADERS")
	if expect == "" {
		return
	}
	for _, name := range []string{"Authorization", "Cookie", "Proxy-Authorization"} {
		values := headers[name]
		if len(values) == 0 {
			fmt.Fprintf(os.Stderr, "%s: missing %s header\n", hook, name)
			os.Exit(2)
		}
		switch expect {
		case "redacted":
			if len(values) != 1 || values[0] != "<redacted>" {
				fmt.Fprintf(os.Stderr, "%s: %s = %q, want redacted\n", hook, name, values)
				os.Exit(2)
			}
		case "preserved":
			if len(values) == 1 && values[0] == "<redacted>" {
				fmt.Fprintf(os.Stderr, "%s: %s was redacted unexpectedly\n", hook, name)
				os.Exit(2)
			}
		}
	}
}
