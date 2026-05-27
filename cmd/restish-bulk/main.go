package main

import (
	"fmt"
	"os"

	"github.com/rest-sh/restish/v2/internal/output"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

func main() {
	pluginwire.Run(
		pluginwire.Manifest{
			Name:              "bulk",
			Version:           "1.0.0",
			Description:       "Git-like bulk resource management for API collections",
			RestishAPIVersion: 2,
			Hooks:             []string{"command"},
		},
		[]pluginwire.CommandDecl{
			{
				Name:  "bulk",
				Short: "Git-like bulk resource management for API resources",
				Long: "Check out collections of remote API resources to disk, track local and remote changes, diff them, and push updates back in bulk.\n\n" +
					"Use `bulk init` on a list endpoint that returns resource URLs and versions. Then use `bulk status`, `bulk diff`, `bulk pull`, and `bulk push` in the checkout directory.",
			},
		},
		func(command string, args []string, base *pluginwire.CommandClient) error {
			if command != "bulk" {
				return fmt.Errorf("unknown command: %s", command)
			}
			term := pluginwire.TerminalContextFromArgs(os.Args[1:])
			if err := output.SetTheme(output.ThemeEntries(term.Theme)); err != nil {
				return err
			}
			client := &pluginClient{
				CommandClient: base,
				term:          term,
			}
			return run(client, args)
		},
	)
}
