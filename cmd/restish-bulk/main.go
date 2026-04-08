package main

import (
	"fmt"
	"os"

	pluginwire "github.com/danielgtaylor/restish/v2/plugin"
)

func main() {
	pluginwire.Run(
		pluginwire.Manifest{
			Name:              "bulk",
			Version:           "1.0.0",
			Description:       "Git-like bulk resource management for API collections",
			RestishAPIVersion: 1,
			Hooks:             []string{"command"},
		},
		[]pluginwire.CommandDecl{
			{
				Name:  "bulk",
				Short: "Git-like bulk resource management for API resources",
				Long:  "Check out collections of remote API resources to disk, track local and remote changes, diff them, and push updates back in bulk.",
			},
		},
		func(command string, args []string, base *pluginwire.CommandClient) error {
			if command != "bulk" {
				return fmt.Errorf("unknown command: %s", command)
			}
			client := &pluginClient{
				CommandClient: base,
				term:          pluginwire.TerminalContextFromArgs(os.Args[1:]),
			}
			return run(client, args)
		},
	)
}
