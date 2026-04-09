// cmdplugin is a test Restish command plugin used in command-plugin tests.
package main

import (
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
)

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--rsh-plugin-manifest":
			manifest := map[string]any{
				"name":                "cmdplugin",
				"version":             "1.0.0",
				"description":         "Test command plugin",
				"restish_api_version": 1,
				"hooks":               []string{"command"},
			}
			data, err := cbor.Marshal(manifest)
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal:", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(data)
			os.Exit(0)
		case "--rsh-plugin-commands":
			resp := map[string]any{
				"commands": []any{
					map[string]any{"name": "greet", "short": "Greet the user"},
					map[string]any{"name": "fetch", "short": "Fetch a URL via Restish"},
					map[string]any{"name": "pipe", "short": "Echo stdin via passthrough stdio", "passthrough_stdio": true},
					map[string]any{"name": "fail", "short": "Exit with code 1"},
					map[string]any{"name": "die", "short": "Crash unexpectedly"},
				},
			}
			data, err := cbor.Marshal(resp)
			if err != nil {
				fmt.Fprintln(os.Stderr, "marshal:", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(data)
			os.Exit(0)
		}
	}

	var initMsg map[string]any
	dec := plugin.NewDecoder(os.Stdin)
	if err := dec.ReadMessage(&initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}
	command, _ := initMsg["command"].(string)

	switch command {
	case "greet":
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "progress", "text": "Greeting in progress..."})
		_ = plugin.WriteMessage(os.Stdout, map[string]any{
			"type":   "response",
			"status": 200,
			"body":   map[string]any{"greeting": "Hello from plugin!"},
		})
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 0})
	case "fetch":
		var fetchURL string
		if args, ok := initMsg["args"].([]any); ok && len(args) > 0 {
			fetchURL, _ = args[0].(string)
		}
		if fetchURL == "" {
			_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 1})
			return
		}
		_ = plugin.WriteMessage(os.Stdout, map[string]any{
			"type":   "http-request",
			"method": "GET",
			"uri":    fetchURL,
		})
		var httpResp map[string]any
		if err := dec.ReadMessage(&httpResp); err != nil {
			fmt.Fprintln(os.Stderr, "read http-response:", err)
			os.Exit(1)
		}
		_ = plugin.WriteMessage(os.Stdout, map[string]any{
			"type":   "response",
			"status": httpResp["status"],
			"body":   httpResp["body"],
		})
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 0})
	case "fail":
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 1})
	case "pipe":
		for {
			var in map[string]any
			if err := dec.ReadMessage(&in); err != nil {
				os.Exit(1)
			}
			switch in["type"] {
			case "stdin-data":
				data, _ := in["data"].([]byte)
				if len(data) == 0 {
					if arr, ok := in["data"].([]any); ok {
						for _, item := range arr {
							if n, ok := item.(uint64); ok {
								data = append(data, byte(n))
							}
						}
					}
				}
				_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "stdout-data", "data": append([]byte("OUT:"), data...)})
				_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "stderr-data", "data": append([]byte("ERR:"), data...)})
			case "stdin-close":
				_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 0})
				return
			}
		}
	case "die":
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"type": "done", "exit_code": 1})
	}
}
