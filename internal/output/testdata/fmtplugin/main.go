// Test formatter plugin for PluginFormatter round-trip tests.
// It outputs each received item as "<event>:<body_json>" lines.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fxamacker/cbor/v2"
	"github.com/rest-sh/restish/v2/plugin"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--rsh-plugin-manifest" {
			data, err := cbor.Marshal(map[string]any{
				"name":                "test-fmt",
				"version":             "1.0.0",
				"description":         "Test formatter plugin",
				"restish_api_version": 2,
				"hooks":               []string{"formatter"},
				"formats":             []string{"testfmt"},
			})
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(data)
			return
		}
	}

	dec := plugin.NewDecoder(os.Stdin)
	for {
		var msg map[string]any
		if err := dec.ReadMessage(&msg); err != nil {
			return
		}
		event, _ := msg["event"].(string)
		body := msg["response"].(map[string]any)["body"]

		b, _ := json.Marshal(body)
		fmt.Fprintf(os.Stdout, "%s:%s\n", event, b)
	}
}
