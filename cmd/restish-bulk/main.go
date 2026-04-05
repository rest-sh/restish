package main

import (
	"fmt"
	"os"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
)

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--rsh-plugin-manifest":
			writeCBOR(manifest())
			return
		case "--rsh-plugin-commands":
			writeCBOR(commands())
			return
		}
	}

	var initMsg map[string]any
	if err := pluginwire.ReadMessage(os.Stdin, &initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}

	client := newPluginClient(os.Stdin, os.Stdout, terminalContextFromArgs(os.Args[1:]))
	if err := run(client, msgStrings(initMsg["args"])); err != nil {
		_ = client.stderr([]byte(err.Error() + "\n"))
		_ = client.done(1)
		return
	}
	_ = client.done(0)
}

func manifest() map[string]any {
	return map[string]any{
		"name":                "bulk",
		"version":             "1.0.0",
		"description":         "Git-like bulk resource management for API collections",
		"restish_api_version": 1,
		"hooks":               []string{"command"},
	}
}

func commands() map[string]any {
	return map[string]any{
		"commands": []any{
			map[string]any{
				"name":  "bulk",
				"short": "Git-like bulk resource management for API resources",
				"long":  "Check out collections of remote API resources to disk, track local and remote changes, diff them, and push updates back in bulk.",
			},
		},
	}
}

func writeCBOR(v any) {
	data, err := cbor.Marshal(v)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(2)
	}
	_, _ = os.Stdout.Write(data)
}
