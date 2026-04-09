package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/filter"
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

// maxBodyBytes returns the response body size cap derived from the
// --rsh-max-body-size flag (MiB). Zero or negative means use the default.
func maxBodyBytes(cmd *cobra.Command) int64 {
	mib, _ := cmd.Flags().GetInt("rsh-max-body-size")
	if mib <= 0 {
		return output.DefaultMaxBodyBytes
	}
	return int64(mib) * 1024 * 1024
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
	return c.runHTTPInternal(cmd, method, args, false, nil, false)
}

// runHTTPInternal is the implementation of runHTTP. followMode=true is used for
// follow-up requests triggered by response-middleware plugins; in that mode,
// response-middleware is skipped to prevent infinite loops.
// extraHeaders holds additional "Name: Value" strings injected by generated
// commands (e.g. OpenAPI header/cookie parameters) that must not be stored on
// the cobra.Command itself since command objects are reused across invocations.
// noAuth strips authentication when following a redirect to a different host,
// preventing credentialed SSRF via a compromised response-middleware plugin.
func (c *CLI) runHTTPInternal(cmd *cobra.Command, method string, args []string, followMode bool, extraHeaders []string, noAuth bool) error {
	rawURL := args[0]
	bodyArgs := args[1:] // positional args after the URL are shorthand body input

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	// Resolve API short names and merge persistent profile settings.
	profileName := c.profileFromCmd(cmd)
	var apiName string

	// Build request body from shorthand args and/or piped stdin.
	stdinIsTTY := output.IsTerminalReader(c.Stdin)
	bodyVal, err := input.Body(c.Stdin, stdinIsTTY, bodyArgs, opts.ContentType)
	if err != nil {
		return fmt.Errorf("building request body: %w", err)
	}

	prepared, err := c.prepareRequest(rawURL, profileName, opts, bodyVal, extraHeaders, noAuth)
	if err != nil {
		return err
	}
	rawURL = prepared.rawURL
	apiName = prepared.apiName
	opts = prepared.opts

	httpResp, err := c.sendPreparedRequest(context.Background(), method, prepared)
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

	resp, err := c.normalizeHTTPResponse(httpResp, maxBodyBytes(cmd))
	if err != nil {
		return err
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
			crossHost := false
			if followU, parseErr := url.Parse(followReq.URI); parseErr == nil {
				crossHost = !strings.EqualFold(followU.Hostname(), httpResp.Request.URL.Hostname())
			}
			if crossHost {
				fmt.Fprintf(c.Stderr, "warning: response-middleware follow to different host %q — stripping credentials\n", followReq.URI)
			}
			return c.runHTTPInternal(cmd, followReq.Method, []string{followReq.URI}, true, nil, crossHost)
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

	filtered, handled, err := c.filterOutput(cmd, filterExpr, doc, lang)
	if err != nil {
		return err
	}

	if filtered == nil && filterExpr != "@" && filterExpr != "body" && filterExpr != "headers" &&
		!strings.HasPrefix(filterExpr, "body.") && !strings.HasPrefix(filterExpr, "headers.") {
		fmt.Fprintf(c.Stderr, "hint: filter returned no results; to access response body fields use 'body.%s'\n", filterExpr)
	}

	if handled {
		return nil
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
	// Copy the map first so we don't mutate the shared c.formatters.
	if fmtName == "table" {
		cols, _ := cmd.Flags().GetString("rsh-columns")
		sortBy, _ := cmd.Flags().GetString("rsh-sort-by")
		tf := &output.TableFormatter{SortBy: sortBy}
		if cols != "" {
			tf.Columns = strings.Split(cols, ",")
		}
		copied := make(map[string]output.Formatter, len(fmts))
		for k, v := range fmts {
			copied[k] = v
		}
		copied["table"] = tf
		fmts = copied
	}

	formatter, ok := output.Select(fmts, fmtName, tty)
	if !ok {
		return fmt.Errorf("unknown output format %q; available: %s", fmtName, output.FormatterNames(fmts))
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

// writeRaw writes value to stdout as plain text (via filter.RawOutput),
// appending a newline if the result does not already end with one.
// Used by both formatResponse and formatStreamItem for --rsh-raw output.
func (c *CLI) writeRaw(value any) error {
	s := filter.RawOutput(value)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	_, err := io.WriteString(c.Stdout, s)
	return err
}

// logVerbose prints request and response summary lines to stderr.
// Sensitive request headers (Authorization, Cookie, Set-Cookie,
// Proxy-Authorization) are redacted to avoid leaking credentials.
func (c *CLI) logVerbose(resp *http.Response) {
	req := resp.Request
	fmt.Fprintf(c.Stderr, "> %s %s\n", req.Method, req.URL)
	for k, vs := range req.Header {
		for _, v := range vs {
			if isSensitiveHeader(k) {
				v = "<redacted>"
			}
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

// isSensitiveHeader reports whether a header name carries credentials and
// should be redacted in verbose output.
func isSensitiveHeader(name string) bool {
	switch http.CanonicalHeaderKey(name) {
	case "Authorization", "Cookie", "Set-Cookie", "Proxy-Authorization":
		return true
	}
	return false
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
		// Fallback: rawURL may be a full URL built from an operation_base prefix.
		// Check if any registered API's operation_base is a prefix of rawURL,
		// and if so apply that API's profile for auth/headers without rewriting
		// the URL (it is already correct).
		for name, apiCfg := range c.cfg.APIs {
			if apiCfg.OperationBase == "" {
				continue
			}
			base := strings.TrimRight(apiCfg.OperationBase, "/")
			if strings.HasPrefix(rawURL, base) {
				var prof *config.ProfileConfig
				if apiCfg.Profiles != nil {
					prof = apiCfg.Profiles[profileName]
				}
				if prof != nil {
					opts.Headers = append(append([]string(nil), prof.Headers...), opts.Headers...)
					opts.Query = append(append([]string(nil), prof.Query...), opts.Query...)
					opts.OnRequest = c.authOnRequest(name, profileName, prof)
					if opts.TLSSignerName == "" {
						opts.TLSSignerName = prof.TLSSigner
					}
					opts.TLSSignerParams = mergeTLSSignerParams(opts.TLSSignerParams, prof.TLSSignerParams)
				} else {
					opts.OnRequest = c.authOnRequest(name, profileName, nil)
				}
				return rawURL, name, opts
			}
		}
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
	if prof == nil && profileName != "default" {
		fmt.Fprintf(c.Stderr, "warning: profile %q not found for API %q; using API defaults\n", profileName, apiName)
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
		if opts.TLSSignerName == "" {
			opts.TLSSignerName = prof.TLSSigner
		}
		opts.TLSSignerParams = mergeTLSSignerParams(opts.TLSSignerParams, prof.TLSSignerParams)
	}

	return expanded, apiName, opts
}

// mergeTLSSignerParams merges src entries into dst, not overwriting existing
// keys. Returns the (possibly newly allocated) dst map.
func mergeTLSSignerParams(dst, src map[string]string) map[string]string {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	for k, v := range src {
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
	return dst
}

func parseKVStrings(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(values))
	for _, item := range values {
		key, value, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%q: expected \"key=value\" format", item)
		}
		out[strings.TrimSpace(key)] = value
	}
	return out, nil
}

// httpOptsFromFlags reads the global HTTP flags from cmd and builds an Options.
func (c *CLI) httpOptsFromFlags(cmd *cobra.Command) (request.Options, error) {
	headers, _ := cmd.Flags().GetStringArray("rsh-header")
	query, _ := cmd.Flags().GetStringArray("rsh-query")
	server, _ := cmd.Flags().GetString("rsh-server")
	insecure, _ := cmd.Flags().GetBool("rsh-insecure")
	if insecure {
		fmt.Fprintln(c.Stderr, "warning: TLS certificate verification is disabled (--rsh-insecure); connections are not secure")
	}
	clientCert, _ := cmd.Flags().GetString("rsh-client-cert")
	clientKey, _ := cmd.Flags().GetString("rsh-client-key")
	tlsSigner, _ := cmd.Flags().GetString("rsh-tls-signer")
	tlsSignerParamsRaw, _ := cmd.Flags().GetStringArray("rsh-tls-signer-param")
	caCert, _ := cmd.Flags().GetString("rsh-ca-cert")
	noCache, _ := cmd.Flags().GetBool("rsh-no-cache")
	tlsMinVersionStr, _ := cmd.Flags().GetString("rsh-tls-min-version")
	tlsMinVersion, err := request.TLSVersionFromString(tlsMinVersionStr)
	if err != nil {
		return request.Options{}, err
	}

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
	tlsSignerParams, err := parseKVStrings(tlsSignerParamsRaw)
	if err != nil {
		return request.Options{}, fmt.Errorf("invalid tls signer param: %w", err)
	}

	return request.Options{
		Headers:              headers,
		Query:                query,
		Server:               server,
		Insecure:             insecure,
		ClientCertPath:       clientCert,
		ClientKeyPath:        clientKey,
		TLSSignerName:        tlsSigner,
		TLSSignerParams:      tlsSignerParams,
		CACertPath:           caCert,
		TLSMinVersion:        tlsMinVersion,
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
