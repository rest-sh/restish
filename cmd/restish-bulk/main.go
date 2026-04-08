package main

import (
	"fmt"
	"os"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
)

func main() {
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--rsh-plugin-manifest":
			if err := pluginwire.WriteManifest(os.Stdout, pluginwire.Manifest{
				Name:              "bulk",
				Version:           "1.0.0",
				Description:       "Git-like bulk resource management for API collections",
				RestishAPIVersion: 1,
				Hooks:             []string{"command"},
			}); err != nil {
				fmt.Fprintln(os.Stderr, "manifest:", err)
				os.Exit(2)
			}
			return
		case "--rsh-plugin-commands":
			if err := pluginwire.WriteCommands(os.Stdout, []pluginwire.CommandDecl{
				{
					Name:  "bulk",
					Short: "Git-like bulk resource management for API resources",
					Long:  "Check out collections of remote API resources to disk, track local and remote changes, diff them, and push updates back in bulk.",
				},
			}); err != nil {
				fmt.Fprintln(os.Stderr, "commands:", err)
				os.Exit(2)
			}
			return
		}
	}

	var initMsg map[string]any
	if err := pluginwire.ReadMessage(os.Stdin, &initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}

	client := newPluginClient(os.Stdin, os.Stdout, terminalContextFromArgs(os.Args[1:]))
	if err := run(client, pluginwire.MsgStrings(initMsg["args"])); err != nil {
		_ = client.Stderr([]byte(err.Error() + "\n"))
		_ = client.Done(1)
		return
	}
	_ = client.Done(0)
}
