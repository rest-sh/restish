package cli

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// GlobalFlags holds the parsed value of every persistent rsh-* flag plus the
// corresponding RSH_* environment-variable override. It is populated once in
// PersistentPreRunE and stored on the command context; command RunE functions
// retrieve it via globalFlagsFromContext.
type GlobalFlags struct {
	Headers          []string
	Query            []string
	Server           string
	OutputFormat     string
	Silent           bool
	Columns          string
	SortBy           string
	ContentType      string
	Filter           string
	FilterLang       string
	HeadersShorthand bool // --rsh-headers
	Raw              bool
	Verbose          int
	Insecure         bool
	ClientCert       string
	ClientKey        string
	TLSSigner        string
	TLSSignerParams  []string
	CACert           string
	TLSMinVersion    string
	IgnoreStatus     bool
	Timeout          string
	Profile          string
	NoCache          bool
	NoBrowser        bool
	Retry            int // -1 means "not set by user"
	MaxEvents        int
	NoPaginate       bool
	Collect          bool
	MaxPages         int
	MaxItems         int
	MaxBodySize      int
}

type globalFlagsContextKey struct{}

// parseGlobalFlags reads persistent flags from cmd (which includes all
// inherited persistent flags from parent commands) and merges RSH_* env vars.
// Env vars have lower precedence than explicit flag values.
func parseGlobalFlags(cmd *cobra.Command) GlobalFlags {
	var gf GlobalFlags

	// StringArray flags
	gf.Headers, _ = cmd.Flags().GetStringArray("rsh-header")
	gf.Query, _ = cmd.Flags().GetStringArray("rsh-query")
	gf.TLSSignerParams, _ = cmd.Flags().GetStringArray("rsh-tls-signer-param")

	// String flags
	gf.Server, _ = cmd.Flags().GetString("rsh-server")
	gf.OutputFormat, _ = cmd.Flags().GetString("rsh-output-format")
	gf.Columns, _ = cmd.Flags().GetString("rsh-columns")
	gf.SortBy, _ = cmd.Flags().GetString("rsh-sort-by")
	gf.ContentType, _ = cmd.Flags().GetString("rsh-content-type")
	gf.Filter, _ = cmd.Flags().GetString("rsh-filter")
	gf.FilterLang, _ = cmd.Flags().GetString("rsh-filter-lang")
	gf.ClientCert, _ = cmd.Flags().GetString("rsh-client-cert")
	gf.ClientKey, _ = cmd.Flags().GetString("rsh-client-key")
	gf.TLSSigner, _ = cmd.Flags().GetString("rsh-tls-signer")
	gf.CACert, _ = cmd.Flags().GetString("rsh-ca-cert")
	gf.TLSMinVersion, _ = cmd.Flags().GetString("rsh-tls-min-version")
	gf.Profile, _ = cmd.Flags().GetString("rsh-profile")
	gf.Timeout, _ = cmd.Flags().GetString("rsh-timeout")

	// Bool flags
	gf.Silent, _ = cmd.Flags().GetBool("rsh-silent")
	gf.HeadersShorthand, _ = cmd.Flags().GetBool("rsh-headers")
	gf.Raw, _ = cmd.Flags().GetBool("rsh-raw")
	gf.Insecure, _ = cmd.Flags().GetBool("rsh-insecure")
	gf.IgnoreStatus, _ = cmd.Flags().GetBool("rsh-ignore-status-code")
	gf.NoCache, _ = cmd.Flags().GetBool("rsh-no-cache")
	gf.NoBrowser, _ = cmd.Flags().GetBool("rsh-no-browser")
	gf.NoPaginate, _ = cmd.Flags().GetBool("rsh-no-paginate")
	gf.Collect, _ = cmd.Flags().GetBool("rsh-collect")

	// Count flag
	gf.Verbose, _ = cmd.Flags().GetCount("rsh-verbose")

	// Int flags
	gf.Retry, _ = cmd.Flags().GetInt("rsh-retry")
	gf.MaxEvents, _ = cmd.Flags().GetInt("rsh-max-events")
	gf.MaxPages, _ = cmd.Flags().GetInt("rsh-max-pages")
	gf.MaxItems, _ = cmd.Flags().GetInt("rsh-max-items")
	gf.MaxBodySize, _ = cmd.Flags().GetInt("rsh-max-body-size")

	// Apply env-var overrides (env var loses to explicit flag).
	// For StringArray flags (header, query), env var appends one value.
	if v := os.Getenv("RSH_HEADER"); v != "" && !cmd.Flags().Changed("rsh-header") {
		gf.Headers = append([]string{v}, gf.Headers...)
	}
	if v := os.Getenv("RSH_QUERY"); v != "" && !cmd.Flags().Changed("rsh-query") {
		gf.Query = append([]string{v}, gf.Query...)
	}
	if v := os.Getenv("RSH_OUTPUT_FORMAT"); v != "" && !cmd.Flags().Changed("rsh-output-format") {
		gf.OutputFormat = v
	}
	if v := os.Getenv("RSH_FILTER"); v != "" && !cmd.Flags().Changed("rsh-filter") {
		gf.Filter = v
	}
	if v := os.Getenv("RSH_INSECURE"); isTruthy(v) && !cmd.Flags().Changed("rsh-insecure") {
		gf.Insecure = true
	}
	if v := os.Getenv("RSH_NO_CACHE"); isTruthy(v) && !cmd.Flags().Changed("rsh-no-cache") {
		gf.NoCache = true
	}
	if v := os.Getenv("RSH_TIMEOUT"); v != "" && !cmd.Flags().Changed("rsh-timeout") {
		gf.Timeout = v
	}
	if v := os.Getenv("RSH_RETRY"); v != "" && !cmd.Flags().Changed("rsh-retry") {
		if n, err := strconv.Atoi(v); err == nil {
			gf.Retry = n
		}
	}
	if v := os.Getenv("RSH_PROFILE"); v != "" && !cmd.Flags().Changed("rsh-profile") {
		gf.Profile = v
	}

	return gf
}

// isTruthy reports whether a string env value means "true".
func isTruthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes"
}

// withGlobalFlags returns a context with gf stored under globalFlagsContextKey.
func withGlobalFlags(ctx context.Context, gf GlobalFlags) context.Context {
	return context.WithValue(ctx, globalFlagsContextKey{}, gf)
}

// globalFlagsFromContext retrieves the GlobalFlags stored on ctx.
// If none are present (e.g. in tests that bypass PersistentPreRunE), it
// falls back to empty GlobalFlags with Retry=-1 (the sentinel for "use default").
func globalFlagsFromContext(ctx context.Context) GlobalFlags {
	if gf, ok := ctx.Value(globalFlagsContextKey{}).(GlobalFlags); ok {
		return gf
	}
	return GlobalFlags{Retry: -1, MaxPages: 25}
}
