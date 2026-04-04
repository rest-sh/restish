package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"net/http"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/filter"
	"github.com/danielgtaylor/restish/v2/internal/hypermedia"
	"github.com/danielgtaylor/restish/v2/internal/input"
	"github.com/danielgtaylor/restish/v2/internal/output"
	"github.com/danielgtaylor/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

// cacheDir returns the effective HTTP response cache directory, checking the
// CachePath override (used in tests), then RSH_CACHE_DIR, then the default.
func (c *CLI) cacheDir() string {
	if c.CachePath != "" {
		return c.CachePath
	}
	return config.DefaultCacheDir()
}

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
	return c.runHTTPInternal(cmd, method, args, false)
}

// runHTTPInternal is the implementation of runHTTP. followMode=true is used for
// follow-up requests triggered by response-middleware plugins; in that mode,
// response-middleware is skipped to prevent infinite loops.
func (c *CLI) runHTTPInternal(cmd *cobra.Command, method string, args []string, followMode bool) error {
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
	var apiName string
	rawURL, apiName, opts = c.applyAPIProfile(rawURL, profileName, opts)

	// Chain request-middleware plugins after auth.
	origOnReq := opts.OnRequest
	opts.OnRequest = func(req *http.Request) error {
		if origOnReq != nil {
			if err := origOnReq(req); err != nil {
				return err
			}
		}
		return c.runRequestMiddlewarePlugins(req)
	}

	// Build request body from shorthand args and/or piped stdin.
	stdinIsTTY := output.IsTerminalReader(c.Stdin)
	bodyVal, err := input.Body(c.Stdin, stdinIsTTY, bodyArgs, opts.ContentType)
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
		encoded, actualContentType, err := c.content.EncodeWithType(mimeType, bodyVal)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
		opts.Headers = append(opts.Headers, "Content-Type: "+actualContentType)
	}

	var reqBody io.Reader
	if bodyReader != nil {
		reqBody = bodyReader
	}
	httpResp, err := request.Do(context.Background(), method, rawURL, reqBody, opts)
	if err != nil {
		return fmt.Errorf("network error for %s %s: %w", method, rawURL, err)
	}

	// Verbose logging to stderr.
	if verbose, _ := cmd.Flags().GetCount("rsh-verbose"); verbose >= 1 && httpResp.Request != nil {
		c.logVerbose(httpResp)
	}

	// Streaming responses (SSE, NDJSON) are handled before body normalization.
	if kind := streamingContentType(httpResp.Header.Get("Content-Type")); kind != "" {
		switch kind {
		case "sse":
			return c.handleSSE(cmd, httpResp)
		case "ndjson":
			return c.handleNDJSON(cmd, httpResp)
		}
	}

	resp, err := output.Normalize(httpResp, c.content)
	if err != nil {
		return err
	}

	// Populate links from hypermedia parsers. httpResp headers/request are still
	// accessible even after Normalize has closed and consumed the body.
	if httpResp.Request != nil {
		if links := hypermedia.Parse(httpResp.Request.URL, httpResp.Header, resp.Body, c.linkParsers); len(links) > 0 {
			resp.Links = make(map[string]any, len(links))
			for k, v := range links {
				resp.Links[k] = v
			}
		}
	}

	// Response-middleware plugins: can modify, drop, or follow.
	// Skipped in follow mode to prevent infinite loops.
	if !followMode && httpResp.Request != nil {
		drop, followReq, mwErr := c.runResponseMiddlewarePlugins(httpResp.Request, resp)
		if mwErr != nil {
			return mwErr
		}
		if drop {
			return nil
		}
		if followReq != nil {
			return c.runHTTPInternal(cmd, followReq.Method, []string{followReq.URI}, true)
		}
	}

	// Pagination: if this is a GET and there's a next link, paginate.
	if method == "GET" {
		var pagCfg *config.PaginationConfig
		if apiName != "" && c.cfg != nil && c.cfg.APIs[apiName] != nil {
			pagCfg = c.cfg.APIs[apiName].Pagination
		}
		if did, err := c.tryPaginate(cmd, resp, rawURL, opts, pagCfg); did {
			return err
		}
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
	// Silent mode: suppress all output.
	if silent, _ := cmd.Flags().GetBool("rsh-silent"); silent {
		return nil
	}

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

	fmts := c.formatters
	if fmts == nil {
		fmts = output.DefaultFormatters()
	}

	// For the table format, configure it from flags before selecting.
	if fmtName == "table" {
		cols, _ := cmd.Flags().GetString("rsh-columns")
		sortBy, _ := cmd.Flags().GetString("rsh-sort-by")
		tf := &output.TableFormatter{SortBy: sortBy}
		if cols != "" {
			tf.Columns = strings.Split(cols, ",")
		}
		fmts["table"] = tf
	}

	formatter, ok := output.Select(fmts, fmtName, tty)
	if !ok {
		return fmt.Errorf("unknown output format %q; available: readable, json, raw, table, gron, cbor", fmtName)
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

// logVerbose prints request and response summary lines to stderr.
func (c *CLI) logVerbose(resp *http.Response) {
	req := resp.Request
	fmt.Fprintf(c.Stderr, "> %s %s\n", req.Method, req.URL)
	for k, vs := range req.Header {
		for _, v := range vs {
			fmt.Fprintf(c.Stderr, "> %s: %s\n", k, v)
		}
	}
	fmt.Fprintln(c.Stderr, ">")
	fmt.Fprintf(c.Stderr, "< %s %d %s\n", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode))
	for k, vs := range resp.Header {
		for _, v := range vs {
			fmt.Fprintf(c.Stderr, "< %s: %s\n", k, v)
		}
	}
	fmt.Fprintln(c.Stderr, "<")
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
// Returns (expandedURL, apiName, opts). apiName is empty when rawURL is not
// an API short name.
func (c *CLI) applyAPIProfile(rawURL, profileName string, opts request.Options) (string, string, request.Options) {
	if c.cfg == nil || len(c.cfg.APIs) == 0 {
		return rawURL, "", opts
	}

	// Split "apiname/rest/of/path" → apiName="apiname", rest="rest/of/path"
	apiName, rest, _ := strings.Cut(rawURL, "/")
	api, ok := c.cfg.APIs[apiName]
	if !ok {
		return rawURL, "", opts
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
		opts.OnRequest = c.authOnRequest(apiName, profileName, prof)
	}

	return expanded, apiName, opts
}

// httpOptsFromFlags reads the global HTTP flags from cmd and builds an Options.
func (c *CLI) httpOptsFromFlags(cmd *cobra.Command) (request.Options, error) {
	headers, _ := cmd.Flags().GetStringArray("rsh-header")
	query, _ := cmd.Flags().GetStringArray("rsh-query")
	server, _ := cmd.Flags().GetString("rsh-server")
	insecure, _ := cmd.Flags().GetBool("rsh-insecure")
	noCache, _ := cmd.Flags().GetBool("rsh-no-cache")

	// --rsh-timeout, falling back to RSH_TIMEOUT env var.
	timeoutStr, _ := cmd.Flags().GetString("rsh-timeout")
	if timeoutStr == "" {
		timeoutStr = os.Getenv("RSH_TIMEOUT")
	}
	var timeout time.Duration
	if timeoutStr != "" {
		var parseErr error
		timeout, parseErr = time.ParseDuration(timeoutStr)
		if parseErr != nil {
			return request.Options{}, fmt.Errorf("invalid timeout %q: %w", timeoutStr, parseErr)
		}
	}

	// --rsh-retry, falling back to RSH_RETRY env var; default is 2 when
	// neither is set.  The flag default is -1 (sentinel = "not set by user").
	retry := 2
	if envVal := os.Getenv("RSH_RETRY"); envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil {
			retry = n
		}
	}
	if flagVal, _ := cmd.Flags().GetInt("rsh-retry"); flagVal >= 0 {
		retry = flagVal
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
		CacheDir:             c.cacheDir(),
		NoCache:              noCache,
		Retry:                retry,
		RetryBaseDelay:       c.RetryBaseDelay,
	}, nil
}
