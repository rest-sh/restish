package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (c *CLI) newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "restish",
		Short: "A CLI for interacting with REST-ish HTTP APIs",
		Long: `Restish is a CLI for interacting with REST-ish HTTP APIs.

Every API deserves a CLI. Restish provides generic HTTP commands for
quick one-off requests, and generates documented, shell-completed
commands for registered APIs via OpenAPI 3.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// ArbitraryArgs prevents cobra's legacyArgs validator from rejecting
		// unrecognised args before our RunE can inspect them (which we need for
		// bare-URL dispatch: "restish https://api.example.com").
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// A bare URL (no explicit verb) is treated as GET.
			// Anything containing . : or / is likely a URL or API short name.
			if strings.ContainsAny(args[0], ".:/") {
				return c.runHTTP(cmd, "GET", args)
			}
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.Name())
		},
	}

	c.addGlobalFlags(root)
	c.addHTTPCommands(root)
	return root
}

// addGlobalFlags registers persistent flags that apply to all commands.
func (c *CLI) addGlobalFlags(root *cobra.Command) {
	pf := root.PersistentFlags()
	pf.StringArrayP("rsh-header", "H", nil, `Request header in "Name: Value" format (repeatable)`)
	pf.StringArrayP("rsh-query", "q", nil, `Query parameter in "key=value" format (repeatable)`)
	pf.StringP("rsh-server", "s", "", "Override scheme://host for all requests (e.g. https://staging.example.com)")
	pf.StringP("rsh-output-format", "o", "", "Output format: readable, json, raw (default: readable on TTY, raw otherwise)")
	pf.StringP("rsh-content-type", "c", "", `Request body content type, e.g. json, yaml, cbor (default: json)`)
	pf.StringP("rsh-filter", "f", "", "Filter/project the response using shorthand or jq (auto-detected)")
	pf.String("rsh-filter-lang", "", "Force filter language: shorthand or jq")
	pf.Bool("rsh-headers", false, "Shorthand for -f headers")
	pf.BoolP("rsh-raw", "r", false, "Raw output: strip quotes from strings, one item per line for arrays")
	pf.Bool("rsh-insecure", false, "Disable TLS certificate verification")
	pf.Bool("rsh-ignore-status-code", false, "Always exit 0 regardless of HTTP status")
	pf.String("rsh-timeout", "", "Request timeout, e.g. 30s")
}
