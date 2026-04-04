// hookplugin is a test Restish plugin that implements the auth,
// request-middleware, and response-middleware hooks for unit tests.
//
// Auth hook: always adds Authorization: Bearer hook-token.
// Request middleware: adds X-Trace-Id: hook-trace-123.
// Response middleware: behaviour is controlled by RSH_HOOK_RM_BEHAVIOR:
//
//	""          → adds "plugin_added":true to the response body (if map)
//	"drop"      → returns {"drop":true}
//	"follow:<u>"→ returns {"follow":{"method":"GET","uri":"<u>"}}
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--rsh-plugin-manifest" {
			manifest := map[string]any{
				"name":                "hookplugin",
				"version":             "1.0.0",
				"description":         "Test hook plugin",
				"restish_api_version": 1,
				"hooks":               []string{"auth", "request-middleware", "response-middleware"},
			}
			data, err := cbor.Marshal(manifest)
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal:", err)
				os.Exit(2)
			}
			os.Stdout.Write(data)
			os.Exit(0)
		}
	}

	// Read one CBOR message from stdin.
	var msg map[string]any
	if err := plugin.ReadMessage(os.Stdin, &msg); err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	hookType, _ := msg["type"].(string)

	switch hookType {
	case "auth":
		out := map[string]any{
			"request": map[string]any{
				"headers": map[string]any{
					"Authorization": []any{"Bearer hook-token"},
				},
			},
		}
		if err := plugin.WriteMessage(os.Stdout, out); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}

	case "request-middleware":
		req, _ := msg["request"].(map[string]any)
		hdrs, _ := req["headers"].(map[string]any)
		if hdrs == nil {
			hdrs = map[string]any{}
		}
		hdrs["X-Trace-Id"] = []any{"hook-trace-123"}
		out := map[string]any{
			"request": map[string]any{
				"method":  req["method"],
				"uri":     req["uri"],
				"headers": hdrs,
			},
		}
		if err := plugin.WriteMessage(os.Stdout, out); err != nil {
			fmt.Fprintln(os.Stderr, "write:", err)
			os.Exit(1)
		}

	case "response-middleware":
		behavior := os.Getenv("RSH_HOOK_RM_BEHAVIOR")
		switch {
		case behavior == "drop":
			out := map[string]any{"drop": true}
			plugin.WriteMessage(os.Stdout, out) //nolint:errcheck
		case strings.HasPrefix(behavior, "follow:"):
			uri := strings.TrimPrefix(behavior, "follow:")
			out := map[string]any{
				"follow": map[string]any{
					"method": "GET",
					"uri":    uri,
				},
			}
			plugin.WriteMessage(os.Stdout, out) //nolint:errcheck
		default:
			// Add plugin_added:true to body map.
			respMsg, _ := msg["response"].(map[string]any)
			body, _ := respMsg["body"].(map[string]any)
			if body == nil {
				body = map[string]any{}
			}
			body["plugin_added"] = true
			out := map[string]any{
				"response": map[string]any{
					"body": body,
				},
			}
			plugin.WriteMessage(os.Stdout, out) //nolint:errcheck
		}

	default:
		// Unknown hook type: exit without writing (no-op).
		os.Exit(0)
	}
}
