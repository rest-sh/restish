package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/filter"
	"github.com/danielgtaylor/restish/v2/internal/input"
	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

// addHTTPCommands registers the generic HTTP verb commands on root.
func (c *CLI) addHTTPCommands(root *cobra.Command) {
	verbs := []struct {
		name  string
		short string
	}{
		{"get", "Perform an HTTP GET request"},
		{"head", "Perform an HTTP HEAD request"},
		{"options", "Perform an HTTP OPTIONS request"},
		{"post", "Perform an HTTP POST request"},
		{"put", "Perform an HTTP PUT request"},
		{"patch", "Perform an HTTP PATCH request"},
		{"delete", "Perform an HTTP DELETE request"},
	}

	for _, v := range verbs {
		v := v
		method := strings.ToUpper(v.name)
		cmd := &cobra.Command{
			Use:     v.name + " <url>",
			Aliases: []string{method},
			Short:   v.short,
			Args:    cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return c.runHTTP(cmd, method, args)
			},
		}
		root.AddCommand(cmd)
	}
}

// runHTTP reads global flags, executes the HTTP request, normalizes the
// response, formats it, and handles exit codes.
func (c *CLI) runHTTP(cmd *cobra.Command, method string, args []string) error {
	rawURL := args[0]
	bodyArgs := args[1:] // positional args after the URL are shorthand body input

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	// Resolve API short names and merge persistent profile headers/query.
	profileName, _ := cmd.Flags().GetString("rsh-profile")
	if profileName == "" {
		profileName = os.Getenv("RSH_PROFILE")
	}
	if profileName == "" {
		profileName = "default"
	}
	rawURL, opts = c.applyAPIProfile(rawURL, profileName, opts)

	// Build request body from shorthand args and/or piped stdin.
	stdinIsTTY := output.IsTerminalReader(c.Stdin)
	bodyVal, err := input.Body(c.Stdin, stdinIsTTY, bodyArgs)
	if err != nil {
		return fmt.Errorf("building request body: %w", err)
	}

	var bodyReader *bytes.Reader
	if bodyVal != nil {
		ct := opts.ContentType
		if ct == "" {
			ct = "application/json"
		}
		// Determine the full MIME type for marshalling.
		mimeType := c.content.MIMETypeForName(ct)
		if mimeType == "" {
			mimeType = ct
		}
		encoded, err := c.content.Encode(mimeType, bodyVal)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
		if opts.ContentType == "" {
			opts.Headers = append(opts.Headers, "Content-Type: "+mimeType)
		}
	}

	var reqBody io.Reader
	if bodyReader != nil {
		reqBody = bodyReader
	}
	httpResp, err := request.Do(context.Background(), method, rawURL, reqBody, opts)
	if err != nil {
		return fmt.Errorf("network: %w", err)
	}

	resp, err := output.Normalize(httpResp, c.content)
	if err != nil {
		return err
	}

	if err := c.formatResponse(cmd, resp); err != nil {
		return err
	}

	ignoreStatus, _ := cmd.Flags().GetBool("rsh-ignore-status-code")
	if !ignoreStatus {
		if code := output.StatusToExitCode(resp.Status); code != 0 {
			return &ExitCodeError{Code: code}
		}
	}
	return nil
}

// formatResponse applies any filter then selects and runs the formatter.
func (c *CLI) formatResponse(cmd *cobra.Command, resp *output.Response) error {
	fmtName, _ := cmd.Flags().GetString("rsh-output-format")
	filterExpr, _ := cmd.Flags().GetString("rsh-filter")
	filterLang, _ := cmd.Flags().GetString("rsh-filter-lang")
	headersOnly, _ := cmd.Flags().GetBool("rsh-headers")
	rawMode, _ := cmd.Flags().GetBool("rsh-raw")
	tty := output.IsTerminal(c.Stdout)

	if headersOnly {
		filterExpr = "headers"
	}

	// Default filter: full response on TTY or when using the readable format;
	// body only on non-TTY with other formats (json/raw/scripting).
	if filterExpr == "" {
		if tty || fmtName == "readable" {
			filterExpr = "@"
		} else {
			filterExpr = "body"
		}
	}

	// Resolve filter language.
	var lang filter.Lang
	switch strings.ToLower(filterLang) {
	case "shorthand":
		lang = filter.LangShorthand
	case "jq":
		lang = filter.LangJQ
	default:
		lang = filter.LangAuto
	}

	// Build the full response map for filtering.
	doc := map[string]any{
		"proto":   resp.Proto,
		"status":  resp.Status,
		"headers": resp.Headers,
		"links":   resp.Links,
		"body":    resp.Body,
	}

	filtered, err := filter.Apply(filterExpr, doc, lang)
	if err != nil {
		return fmt.Errorf("filter: %w", err)
	}

	// --rsh-raw: write plain text without encoding.
	if rawMode {
		s := filter.RawOutput(filtered)
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		_, err := io.WriteString(c.Stdout, s)
		return err
	}

	// If the filter selected a sub-value (not the full response), wrap it in
	// a minimal Response so formatters have something to work with.
	var outResp *output.Response
	if filterExpr == "@" {
		outResp = resp
	} else {
		outResp = &output.Response{Body: filtered}
	}

	fmts := output.DefaultFormatters()
	formatter, ok := output.Select(fmts, fmtName, tty)
	if !ok {
		return fmt.Errorf("unknown output format %q; available: readable, json, raw", fmtName)
	}

	// For non-TTY filtered output, use JSON formatter (not raw bytes) since
	// the filtered value is a Go value, not the original wire bytes.
	if !tty && fmtName == "" && filterExpr != "@" {
		encoded, err := json.Marshal(filtered)
		if err != nil {
			return err
		}
		encoded = append(encoded, '\n')
		_, err = c.Stdout.Write(encoded)
		return err
	}

	return formatter.Format(c.Stdout, outResp, output.ColorEnabled(c.Stdout))
}

// isAPIShortName reports whether arg (with no path separator) exactly matches a
// registered API name in the config.
func (c *CLI) isAPIShortName(arg string) bool {
	return c.cfg != nil && c.cfg.APIs[arg] != nil
}

// applyAPIProfile checks whether rawURL begins with a registered API short
// name and, if so, expands it to the full URL and prepends persistent headers
// and query params from the active profile.
//
// If rawURL is not an API short name it is returned unchanged.
func (c *CLI) applyAPIProfile(rawURL, profileName string, opts request.Options) (string, request.Options) {
	if c.cfg == nil || len(c.cfg.APIs) == 0 {
		return rawURL, opts
	}

	// Split "apiname/rest/of/path" → apiName="apiname", rest="rest/of/path"
	apiName, rest, _ := strings.Cut(rawURL, "/")
	api, ok := c.cfg.APIs[apiName]
	if !ok {
		return rawURL, opts
	}

	// Determine effective base URL and profile.
	baseURL := api.BaseURL
	var prof *config.ProfileConfig
	if api.Profiles != nil {
		prof = api.Profiles[profileName]
		if prof != nil && prof.BaseURL != "" {
			baseURL = prof.BaseURL
		}
	}

	// Build the expanded URL.
	expanded := strings.TrimRight(baseURL, "/")
	if rest != "" {
		expanded = expanded + "/" + rest
	}

	// Prepend persistent profile headers/query so flag-supplied values take
	// precedence (they appear later in the slice, and are applied last).
	if prof != nil {
		opts.Headers = append(append([]string(nil), prof.Headers...), opts.Headers...)
		opts.Query = append(append([]string(nil), prof.Query...), opts.Query...)
		opts.OnRequest = c.authOnRequest(prof)
	}

	return expanded, opts
}

// httpOptsFromFlags reads the global HTTP flags from cmd and builds an Options.
func (c *CLI) httpOptsFromFlags(cmd *cobra.Command) (request.Options, error) {
	headers, _ := cmd.Flags().GetStringArray("rsh-header")
	query, _ := cmd.Flags().GetStringArray("rsh-query")
	server, _ := cmd.Flags().GetString("rsh-server")
	insecure, _ := cmd.Flags().GetBool("rsh-insecure")
	timeoutStr, _ := cmd.Flags().GetString("rsh-timeout")

	var timeout time.Duration
	if timeoutStr != "" {
		var parseErr error
		timeout, parseErr = time.ParseDuration(timeoutStr)
		if parseErr != nil {
			return request.Options{}, fmt.Errorf("invalid --rsh-timeout %q: %w", timeoutStr, parseErr)
		}
	}

	contentType, _ := cmd.Flags().GetString("rsh-content-type")

	return request.Options{
		Headers:              headers,
		Query:                query,
		Server:               server,
		Insecure:             insecure,
		Timeout:              timeout,
		AcceptHeader:         c.content.AcceptHeader(),
		AcceptEncodingHeader: c.content.AcceptEncodingHeader(),
		ContentType:          contentType,
	}, nil
}
