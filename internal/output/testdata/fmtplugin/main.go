// Test formatter plugin for PluginFormatter round-trip tests.
// It outputs each received item as "<event>:<body_json>" lines.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rest-sh/restish/v2/plugin"
)

func main() {
	manifest := plugin.Manifest{
		Name:                      "test-fmt",
		Version:                   "1.0.0",
		Description:               "Test formatter plugin",
		RestishAPIVersion:         2,
		Hooks:                     []string{"formatter"},
		FormatterNames:            []string{"testfmt"},
		InteractiveFormatterNames: []string{"testfmt-live"},
	}
	if plugin.HandleStartupFlags(os.Stdout, manifest, nil) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)
	for {
		var msg map[string]any
		if err := dec.ReadMessage(&msg); err != nil {
			return
		}
		messageType, _ := msg["type"].(string)
		if messageType == plugin.MsgTypeTerminalResize {
			fmt.Fprintf(os.Stdout, "resize:%vx%v\n", msg["columns"], msg["rows"])
			continue
		}
		if messageType == plugin.MsgTypeStdinData {
			if data, ok := msg["data"].([]byte); ok && bytes.Contains(data, []byte("q")) {
				fmt.Fprintln(os.Stdout, "quit")
				return
			}
			continue
		}
		event, _ := msg["event"].(string)
		response, _ := msg["response"].(map[string]any)
		body := response["body"]

		b, _ := json.Marshal(body)
		fmt.Fprintf(os.Stdout, "%s:%s\n", event, b)
	}
}
