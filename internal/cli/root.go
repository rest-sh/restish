package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/spf13/cobra"
)

const (
	rootGroupHTTP    = "http"
	rootGroupConfig  = "config"
	rootGroupPlugin  = "plugin"
	rootGroupAPI     = "api"
	rootGroupUtility = "utility"
	rootGroupHelp    = "help"
)

const securityCompletionAnnotation = "restish.securityCompletions"

func (c *CLI) newRootCmd() *cobra.Command {
	use := c.commandName
	if use == "" {
		use = "restish"
	}
	short := c.commandShort
	if short == "" {
		short = "A CLI for interacting with REST-ish HTTP APIs"
	}
	long := c.commandLong
	if long == "" {
		long = `Restish is a CLI for interacting with REST-ish HTTP APIs.

Every API deserves a CLI. Restish provides generic HTTP commands for
quick one-off requests, and generates documented, shell-completed
commands for registered APIs via OpenAPI 3.`
	}
	root := &cobra.Command{
		Use:           use,
		Short:         short,
		Long:          long,
		Version:       c.currentVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
		// ArbitraryArgs prevents cobra's legacyArgs validator from rejecting
		// unrecognised args before our RunE can inspect them (which we need for
		// bare-URL dispatch: "restish https://api.example.com").
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: c.completeRootURL,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			gf, err := parseGlobalFlags(cmd)
			if err != nil {
				return err
			}
			cmd.SetContext(withGlobalFlags(cmd.Context(), gf))
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// A bare URL (no explicit verb) is treated as GET.
			// Anything containing . : or / is likely a URL; a word matching a
			// registered API name is also routed as a GET request.
			if strings.ContainsAny(args[0], ".:/") || c.isAPIShortName(args[0]) {
				return c.runHTTP(cmd, "GET", args)
			}
			return fmt.Errorf("unknown command %q for %q; run %q to see available commands or use a full URL", args[0], cmd.Name(), cmd.CommandPath()+" --help")
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return newUsageError(err)
	})

	addRootCommandGroups(root)
	setupGroupedUsage(root)
	c.addGlobalFlags(root)
	c.addHTTPCommands(root)
	c.addEditCommand(root)
	c.addCertCommand(root)
	c.addAPICommand(root)
	c.addCacheCommand(root)
	c.addConfigCommand(root)
	c.addContentTypesCommand(root)
	c.addCompletionCommand(root)
	c.addShellCommand(root)
	c.addLinksCommand(root)
	c.addFlagsCommand(root)
	c.addVersionCommand(root)
	c.addDoctorCommand(root)
	c.addPluginCommand(root)
	c.addCommandPlugins(root)
	c.setupMarkdownHelp(root)
	return root
}

func (c *CLI) addVersionCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "version",
		Short:   "Print the Restish version",
		GroupID: rootGroupUtility,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(c.Stdout, c.currentVersion())
			return nil
		},
	})
}

func (c *CLI) currentVersion() string {
	if c.commandVersion != "" {
		return c.commandVersion
	}
	return Version
}

func addRootCommandGroups(root *cobra.Command) {
	root.AddGroup(
		&cobra.Group{ID: rootGroupHTTP, Title: "Generic HTTP Commands"},
		&cobra.Group{ID: rootGroupConfig, Title: "Configuration and Setup"},
		&cobra.Group{ID: rootGroupPlugin, Title: "Plugin Commands"},
		&cobra.Group{ID: rootGroupAPI, Title: "Registered APIs"},
		&cobra.Group{ID: rootGroupUtility, Title: "Utilities"},
		&cobra.Group{ID: rootGroupHelp, Title: "Help"},
	)
	root.SetHelpCommandGroupID(rootGroupHelp)
	root.SetCompletionCommandGroupID(rootGroupHelp)
}

func rootCommandHasGroup(root *cobra.Command, id string) bool {
	for _, group := range root.Groups() {
		if group.ID == id {
			return true
		}
	}
	return false
}

// addGlobalFlags registers persistent flags that apply to all commands.
func (c *CLI) addGlobalFlags(root *cobra.Command) {
	pf := root.PersistentFlags()
	pf.StringArrayP("rsh-header", "H", nil, `Request header in "Name: Value" format (repeatable)`)
	pf.StringArrayP("rsh-query", "q", nil, `Query parameter in "key=value" format (repeatable)`)
	pf.StringP("rsh-server", "s", "", "Override scheme://host for all requests (e.g. https://staging.example.com)")
	pf.StringP("rsh-output-format", "o", "", "Output format: "+output.FormatterNames(c.formatters)+" (default: readable on TTY, JSON for structured non-TTY output; use -o lines for shell-friendly filtered values; see --rsh-columns, --rsh-sort-by for table)")
	pf.BoolP("rsh-silent", "S", false, "Suppress all output; only the exit code conveys success or failure")
	pf.String("rsh-columns", "", "Comma-separated column names for -o table (e.g. id,name,status)")
	pf.String("rsh-sort-by", "", "Sort -o table rows by this column name")
	pf.StringP("rsh-content-type", "c", "", `Request body content type, e.g. json, yaml, cbor (default: json)`)
	pf.StringP("rsh-filter", "f", "", "Filter/project the response using shorthand or jq (auto-detected)")
	pf.String("rsh-filter-lang", "", "Force filter language: shorthand or jq")
	pf.Bool("rsh-headers", false, "Shorthand for -f headers")
	pf.BoolP("rsh-raw", "r", false, "Raw output: original response body bytes only")
	pf.CountP("rsh-verbose", "v", "Verbose output: -v shows request/response headers, -vv adds TLS details")
	pf.Bool("rsh-insecure", false, "Disable TLS certificate verification")
	pf.String("rsh-client-cert", "", "Path to a PEM encoded client certificate for mTLS")
	pf.String("rsh-client-key", "", "Path to a PEM encoded private key for mTLS")
	pf.String("rsh-tls-signer", "", "TLS signer plugin to use for mTLS client certificate signing")
	pf.StringArray("rsh-tls-signer-param", nil, `TLS signer plugin parameter in "key=value" format (repeatable)`)
	pf.String("rsh-ca-cert", "", "Path to a PEM encoded CA certificate to trust")
	pf.String("rsh-tls-min-version", "", "Minimum TLS version: TLS1.2 or TLS1.3")
	pf.Bool("rsh-ignore-status-code", false, "Always exit 0 regardless of HTTP status")
	pf.StringP("rsh-timeout", "t", "", "Request timeout, e.g. 30s")
	pf.StringP("rsh-profile", "p", "", "API profile to use (overrides RSH_PROFILE env var; default: \"default\")")
	pf.String("rsh-auth", "", `Generated operation auth override, e.g. "PartnerKey" or "UserOAuth+PartnerKey"`)
	pf.Bool("rsh-no-cache", false, "Bypass the HTTP response cache (no read, no write)")
	pf.Bool("rsh-no-browser", false, "Disable automatic browser launch for interactive auth flows")
	pf.Int("rsh-retry", -1, "Maximum retry attempts for network errors and transient HTTP responses (default: 2; 0 = disable)")
	if flag := pf.Lookup("rsh-retry"); flag != nil {
		flag.DefValue = ""
	}
	pf.Bool("rsh-retry-unsafe", false, "Allow retries for POST, PUT, PATCH, and DELETE requests")
	pf.String("rsh-retry-max-wait", "", "Maximum wait for Retry-After/X-Retry-In delays (default: 5m)")
	pf.Int("rsh-max-events", 1000, "Maximum number of SSE events or NDJSON lines to process (0 = unlimited)")
	pf.Bool("rsh-no-paginate", false, "Disable automatic pagination (return only the first page)")
	pf.Bool("rsh-collect", false, "Collect all pages then apply filter (default: stream items as they arrive)")
	pf.Int("rsh-max-pages", 25, "Maximum number of pages to fetch (0 = unlimited)")
	pf.Int("rsh-max-items", 0, "Maximum number of items to collect across all pages (0 = unlimited)")
	pf.Int("rsh-max-body-size", 0, fmt.Sprintf("Maximum response body size in MiB (0 = default %d MiB)", output.DefaultMaxBodyBytes/(1024*1024)))
	pf.String("rsh-config", "", "Path to the restish config file (overrides RSH_CONFIG and the platform default)")
	pf.Bool("help-all", false, "Show all inherited Restish flags in help")

	c.registerFlagCompletions(root)
}

// registerFlagCompletions installs shell-completion functions for the global
// persistent flags that benefit from dynamic or well-known value suggestions.
func (c *CLI) registerFlagCompletions(root *cobra.Command) {
	// -o / --rsh-output-format: static list from registered formatters.
	_ = root.RegisterFlagCompletionFunc("rsh-output-format", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		fmts := c.formatters
		if fmts == nil {
			fmts = output.DefaultFormatters()
		}
		names := make([]string, 0, len(fmts))
		for name := range fmts {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, cobra.ShellCompDirectiveNoFileComp
	})

	// -p / --rsh-profile: dynamic list from all API profiles in config.
	_ = root.RegisterFlagCompletionFunc("rsh-profile", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		if c.cfg == nil {
			return []string{"default"}, cobra.ShellCompDirectiveNoFileComp
		}
		seen := map[string]struct{}{"default": {}}
		for _, api := range c.cfg.APIs {
			for name := range api.Profiles {
				seen[name] = struct{}{}
			}
		}
		names := make([]string, 0, len(seen))
		for name := range seen {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, cobra.ShellCompDirectiveNoFileComp
	})

	_ = root.RegisterFlagCompletionFunc("rsh-auth", func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		for current := cmd; current != nil; current = current.Parent() {
			if current.Annotations == nil {
				continue
			}
			raw := current.Annotations[securityCompletionAnnotation]
			if raw == "" {
				continue
			}
			return strings.Split(raw, "\n"), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	})

	// -c / --rsh-content-type: dynamic list from registered content types.
	_ = root.RegisterFlagCompletionFunc("rsh-content-type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		if c.content == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var names []string
		for _, ct := range c.content.ContentTypes() {
			if ct.Name != "" {
				names = append(names, ct.Name)
			}
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})

	// --rsh-filter-lang: static list of supported filter languages.
	_ = root.RegisterFlagCompletionFunc("rsh-filter-lang", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"shorthand", "jq"}, cobra.ShellCompDirectiveNoFileComp
	})
}
