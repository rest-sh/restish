package cli

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/rest-sh/restish/v2/internal/input"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

// cacheDir returns the effective HTTP response cache directory, checking the
// CachePath override (used in tests), then RSH_CACHE_DIR, then the default.
func (c *CLI) cacheDir() string {
	if c.hooks.CachePath != "" {
		return c.hooks.CachePath
	}
	return c.paths().Cache()
}

// maxBodyBytes returns the response body size cap derived from the
// --rsh-max-body-size flag (MiB). Zero or negative means use the default.
func maxBodyBytes(cmd *cobra.Command) int64 {
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.MaxBodySize <= 0 {
		return output.DefaultMaxBodyBytes
	}
	return int64(gf.MaxBodySize) * 1024 * 1024
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
			GroupID: rootGroupHTTP,
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
	return c.runHTTPInternal(cmd, method, args, false, nil, false, "")
}

// runHTTPInternal is the implementation of runHTTP. followMode=true is used for
// follow-up requests triggered by response-middleware plugins; in that mode,
// response-middleware is skipped to prevent infinite loops.
// extraHeaders holds additional "Name: Value" strings injected by generated
// commands (e.g. OpenAPI header/cookie parameters) that must not be stored on
// the cobra.Command itself since command objects are reused across invocations.
// noAuth strips authentication when following a redirect to a different host,
// preventing credentialed SSRF via a compromised response-middleware plugin.
func (c *CLI) runHTTPInternal(cmd *cobra.Command, method string, args []string, followMode bool, extraHeaders []string, noAuth bool, firstPartyHost string) error {
	rawURL := args[0]
	bodyArgs := args[1:] // positional args after the URL are shorthand body input

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}

	// Resolve API short names and merge persistent profile settings.
	profileName := c.profileFromCmd(cmd)
	authOpts, err := c.authHandlerOptionsFromCmd(cmd)
	if err != nil {
		return err
	}
	var apiName string

	// Build request body from shorthand args and/or piped stdin.
	stdinIsTTY := output.IsTerminalReader(c.Stdin)
	bodyVal, err := input.Body(c.Stdin, stdinIsTTY, bodyArgs, opts.ContentType)
	if err != nil {
		return fmt.Errorf("building request body: %w", err)
	}

	prepared, err := c.prepareRequest(rawURL, profileName, opts, bodyVal, extraHeaders, noAuth, authOpts)
	if err != nil {
		return err
	}
	defer c.closePreparedTransport(prepared)
	rawURL = prepared.rawURL
	apiName = prepared.apiName
	opts = prepared.opts
	if firstPartyHost == "" {
		if u, parseErr := url.Parse(prepared.rawURL); parseErr == nil {
			firstPartyHost = u.Hostname()
		}
	}

	httpResp, err := c.sendPreparedRequest(requestContext(cmd), method, prepared)
	if err != nil {
		return fmt.Errorf("network error for %s %s: %w", method, rawURL, err)
	}

	// Verbose logging to stderr.
	if v := globalFlagsFromContext(requestContext(cmd)).Verbose; v >= 1 && httpResp.Request != nil {
		c.logVerbose(httpResp, v)
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
			crossHost := followCrossesFirstPartyHost(firstPartyHost, followReq.URI)
			if crossHost {
				fmt.Fprintf(c.Stderr, "warning: response-middleware follow to different host %q — stripping credentials\n", followReq.URI)
			}
			return c.runHTTPInternal(cmd, followReq.Method, []string{followReq.URI}, true, nil, crossHost, firstPartyHost)
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

	ignoreStatus := globalFlagsFromContext(requestContext(cmd)).IgnoreStatus
	if !ignoreStatus {
		if code := output.StatusToExitCode(resp.Status); code != 0 {
			return &ExitCodeError{Code: code}
		}
	}
	return nil
}

func followCrossesFirstPartyHost(firstPartyHost, followURI string) bool {
	if firstPartyHost == "" {
		return false
	}
	followU, err := url.Parse(followURI)
	if err != nil {
		return false
	}
	return !strings.EqualFold(followU.Hostname(), firstPartyHost)
}

// formatResponse applies any filter then selects and runs the formatter.
func (c *CLI) formatResponse(cmd *cobra.Command, resp *output.Response) error {
	// Silent mode: suppress all output.
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.Silent {
		return nil
	}

	fmtName := gf.OutputFormat
	filterExpr := gf.Filter
	filterLang := gf.FilterLang
	headersOnly := gf.HeadersShorthand
	tty := output.IsTerminal(c.Stdout)

	if headersOnly && filterExpr != "" {
		fmt.Fprintln(c.Stderr, `warning: --rsh-headers overrides -f; using "headers"`)
	}
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

	if filtered == nil && shouldSuggestBodyPrefix(filterExpr) {
		fmt.Fprintf(c.Stderr, "hint: filter returned no results; to access response body fields use 'body.%s'\n", filterExpr)
	}

	if handled {
		return nil
	}

	if filterExpr != "@" {
		return c.renderValue(cmd, filtered)
	}

	formatter, err := c.selectFormatter(cmd, fmtName, tty, filterExpr)
	if err != nil {
		return err
	}

	return formatter.Format(c.Stdout, resp, output.ColorEnabled(c.Stdout))
}

// renderValue writes a filtered/subselected value using the same formatter
// selection rules as normal responses, but without HTTP status/header preamble.
func (c *CLI) renderValue(cmd *cobra.Command, value any) error {
	renderer, err := c.newValueRenderer(cmd, nil)
	if err != nil {
		return err
	}
	defer renderer.Close()
	return renderer.Render(value)
}

type valueRenderer interface {
	Render(value any) error
	Close() error
}

type valueRendererFunc struct {
	render func(value any) error
}

func (r valueRendererFunc) Render(value any) error {
	return r.render(value)
}

func (r valueRendererFunc) Close() error {
	return nil
}

type valueStreamRenderer struct {
	stream output.ValueStream
}

func (r valueStreamRenderer) Render(value any) error {
	return r.stream.WriteValue(value)
}

func (r valueStreamRenderer) Close() error {
	return r.stream.Close()
}

func (c *CLI) newValueRenderer(cmd *cobra.Command, base *output.Response) (valueRenderer, error) {
	if base == nil {
		base = &output.Response{}
	}

	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.Silent {
		return valueRendererFunc{render: func(any) error { return nil }}, nil
	}

	rawMode := gf.Raw
	if rawMode {
		return valueRendererFunc{render: c.writeRaw}, nil
	}

	fmtName := gf.OutputFormat
	tty := output.IsTerminal(c.Stdout)
	color := output.ColorEnabled(c.Stdout)

	// Body/sub-value rendering should stay machine-friendly by default on
	// non-TTY, even though the default formatter there is `raw`.
	if !tty && fmtName == "" {
		return valueRendererFunc{render: func(value any) error {
			encoded, err := json.Marshal(value)
			if err != nil {
				return err
			}
			encoded = append(encoded, '\n')
			_, err = c.Stdout.Write(encoded)
			return err
		}}, nil
	}

	// Readable output for sub-values omits the synthetic HTTP preamble and just
	// pretty-prints the value.
	if fmtName == "readable" || (fmtName == "" && tty) {
		return valueRendererFunc{render: func(value any) error {
			encoded, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return err
			}
			encoded = append(encoded, '\n')
			if color {
				highlighted, err := output.HighlightWithLexer(output.ReadableLexer, encoded)
				if err == nil {
					_, err = c.Stdout.Write(highlighted)
					return err
				}
			}
			_, err = c.Stdout.Write(encoded)
			return err
		}}, nil
	}

	formatter, err := c.selectFormatter(cmd, fmtName, tty, "")
	if err != nil {
		return nil, err
	}
	if streamFormatter, ok := formatter.(output.ValueStreamFormatter); ok {
		stream, err := streamFormatter.StartValueStream(c.Stdout, base, color)
		if err != nil {
			return nil, err
		}
		if stream != nil {
			return valueStreamRenderer{stream: stream}, nil
		}
	}
	if valueFormatter, ok := formatter.(output.ValueFormatter); ok {
		return valueRendererFunc{render: func(value any) error {
			return valueFormatter.FormatValue(c.Stdout, value, color)
		}}, nil
	}

	return valueRendererFunc{render: func(value any) error {
		resp := &output.Response{
			Proto:   base.Proto,
			Status:  base.Status,
			Headers: base.Headers,
			Links:   base.Links,
			Body:    value,
		}
		return formatter.Format(c.Stdout, resp, color)
	}}, nil
}

func (c *CLI) selectFormatter(cmd *cobra.Command, fmtName string, tty bool, filterExpr string) (output.Formatter, error) {
	fmts := c.formatters
	if fmts == nil {
		fmts = output.DefaultFormatters()
	}

	// For the table format, configure it from flags before selecting.
	// Copy the map first so we don't mutate the shared c.formatters.
	if fmtName == "table" {
		gfTable := globalFlagsFromContext(requestContext(cmd))
		cols := gfTable.Columns
		sortBy := gfTable.SortBy
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

	var (
		formatter output.Formatter
		ok        bool
	)
	if fmtName == "" {
		formatter, ok = output.SelectDefault(fmts, tty, filterExpr)
	} else {
		formatter, ok = output.Select(fmts, fmtName, tty)
	}
	if !ok {
		return nil, fmt.Errorf("unknown output format %q; available: %s", fmtName, output.FormatterNames(fmts))
	}
	return formatter, nil
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

func shouldSuggestBodyPrefix(filterExpr string) bool {
	filterExpr = strings.TrimSpace(filterExpr)
	if filterExpr == "" || strings.HasPrefix(filterExpr, "@") {
		return false
	}
	roots := []string{"body", "headers", "links", "status", "proto"}
	for _, root := range roots {
		if filterExpr == root ||
			strings.HasPrefix(filterExpr, root+".") ||
			strings.HasPrefix(filterExpr, root+"[") ||
			strings.HasPrefix(filterExpr, "."+root) {
			return false
		}
	}
	return true
}

// logVerbose prints request and response summary lines to stderr.
// Sensitive request headers (Authorization, Cookie, Set-Cookie,
// Proxy-Authorization) are redacted to avoid leaking credentials.
// At verbose >= 2 it also dumps TLS version, cipher suite, and peer
// certificate chain (subject, issuer, expiry).
func (c *CLI) logVerbose(resp *http.Response, verbose int) {
	req := resp.Request
	fmt.Fprintf(c.Stderr, "> %s %s\n", req.Method, redactedRequestURL(req.URL))
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

	if verbose >= 2 && resp.TLS != nil {
		tlsVersionName := map[uint16]string{
			tls.VersionTLS10: "TLS 1.0",
			tls.VersionTLS11: "TLS 1.1",
			tls.VersionTLS12: "TLS 1.2",
			tls.VersionTLS13: "TLS 1.3",
		}
		ver, ok := tlsVersionName[resp.TLS.Version]
		if !ok {
			ver = fmt.Sprintf("TLS 0x%04x", resp.TLS.Version)
		}
		fmt.Fprintf(c.Stderr, "* TLS: %s %s\n", ver, tls.CipherSuiteName(resp.TLS.CipherSuite))
		for i, cert := range resp.TLS.PeerCertificates {
			label := "Leaf"
			if i > 0 {
				label = fmt.Sprintf("Chain %d", i)
			}
			fmt.Fprintf(c.Stderr, "* %s Subject: %s\n", label, cert.Subject)
			fmt.Fprintf(c.Stderr, "* %s Issuer: %s\n", label, cert.Issuer)
			fmt.Fprintf(c.Stderr, "* %s Expiry: %s (%s)\n", label, cert.NotAfter.Format(time.RFC3339), relativeExpiry(cert.NotAfter))
		}
	}
}

func redactedRequestURL(u *url.URL) string {
	if u == nil {
		return ""
	}
	copyURL := *u
	q := copyURL.Query()
	for key := range q {
		if isSensitiveQueryParam(key) {
			q.Set(key, "<redacted>")
		}
	}
	copyURL.RawQuery = q.Encode()
	return copyURL.String()
}

func isSensitiveQueryParam(name string) bool {
	name = strings.ToLower(name)
	sensitive := []string{
		"access_token",
		"refresh_token",
		"token",
		"api_key",
		"apikey",
		"client_secret",
		"password",
		"secret",
	}
	for _, key := range sensitive {
		if name == key {
			return true
		}
	}
	return false
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
func (c *CLI) applyAPIProfile(rawURL, profileName string, opts request.Options, authOpts authHandlerOptions) (string, string, request.Options, error) {
	if c.cfg == nil || len(c.cfg.APIs) == 0 {
		return rawURL, "", opts, nil
	}

	// Split "apiname/rest/of/path" → apiName="apiname", rest="rest/of/path"
	apiName, rest, _ := strings.Cut(rawURL, "/")
	api, ok := c.cfg.APIs[apiName]
	if !ok {
		// Fallback: rawURL may be a full URL built from an operation_base prefix.
		// Check if any registered API's operation_base is a prefix of rawURL,
		// and if so apply that API's profile for auth/headers without rewriting
		// the URL (it is already correct).
		type candidate struct {
			name   string
			apiCfg *config.APIConfig
			base   string
		}
		var candidates []candidate
		for name, apiCfg := range c.cfg.APIs {
			if apiCfg.OperationBase == "" {
				continue
			}
			base := strings.TrimRight(apiCfg.OperationBase, "/")
			if strings.HasPrefix(rawURL, base) {
				candidates = append(candidates, candidate{name: name, apiCfg: apiCfg, base: base})
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return len(candidates[i].base) > len(candidates[j].base)
		})
		for _, match := range candidates {
			var prof *config.ProfileConfig
			if match.apiCfg.Profiles != nil {
				prof = match.apiCfg.Profiles[profileName]
			}
			if prof != nil {
				opts.Headers = append(append([]string(nil), prof.Headers...), opts.Headers...)
				opts.Query = append(append([]string(nil), prof.Query...), opts.Query...)
				callbacks := c.authOnRequest(match.name, profileName, prof, authOpts)
				opts.OnRequest = callbacks.OnRequest
				opts.OnUnauthorized = callbacks.OnUnauthorized
				if opts.TLSSignerName == "" {
					opts.TLSSignerName = prof.TLSSigner
				}
				opts.TLSSignerParams = mergeTLSSignerParams(opts.TLSSignerParams, prof.TLSSignerParams)
			} else {
				if match.apiCfg.Profiles != nil {
					return rawURL, match.name, opts, fmt.Errorf("profile %q not found for API %q", profileName, match.name)
				}
				callbacks := c.authOnRequest(match.name, profileName, nil, authOpts)
				opts.OnRequest = callbacks.OnRequest
				opts.OnUnauthorized = callbacks.OnUnauthorized
			}
			return rawURL, match.name, opts, nil
		}
		return rawURL, "", opts, nil
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
	if prof == nil {
		if api.Profiles != nil {
			return rawURL, apiName, opts, fmt.Errorf("profile %q not found for API %q", profileName, apiName)
		}
		callbacks := c.authOnRequest(apiName, profileName, nil, authOpts)
		opts.OnRequest = callbacks.OnRequest
		opts.OnUnauthorized = callbacks.OnUnauthorized
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
		callbacks := c.authOnRequest(apiName, profileName, prof, authOpts)
		opts.OnRequest = callbacks.OnRequest
		opts.OnUnauthorized = callbacks.OnUnauthorized
		if opts.TLSSignerName == "" {
			opts.TLSSignerName = prof.TLSSigner
		}
		opts.TLSSignerParams = mergeTLSSignerParams(opts.TLSSignerParams, prof.TLSSignerParams)
	}
	if apiName != "" {
		opts.CacheNamespace = apiName + ":" + profileName
	}

	return expanded, apiName, opts, nil
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
	gf := globalFlagsFromContext(requestContext(cmd))

	if gf.Insecure {
		fmt.Fprintln(c.Stderr, "warning: TLS certificate verification is disabled (--rsh-insecure); connections are not secure")
	}

	tlsMinVersion, err := request.TLSVersionFromString(gf.TLSMinVersion)
	if err != nil {
		return request.Options{}, err
	}

	// --rsh-timeout default is 2 when not set by user (flag sentinel = -1).
	var timeout time.Duration
	if gf.Timeout != "" {
		var parseErr error
		timeout, parseErr = time.ParseDuration(gf.Timeout)
		if parseErr != nil {
			return request.Options{}, fmt.Errorf("invalid timeout %q: %w", gf.Timeout, parseErr)
		}
	}

	// --rsh-retry default is 2 when not set by user (flag sentinel = -1).
	retry := 2
	if gf.Retry >= 0 {
		retry = gf.Retry
	}

	tlsSignerParams, err := parseKVStrings(gf.TLSSignerParams)
	if err != nil {
		return request.Options{}, fmt.Errorf("invalid tls signer param: %w", err)
	}

	return request.Options{
		Headers:              gf.Headers,
		Query:                gf.Query,
		Server:               gf.Server,
		Insecure:             gf.Insecure,
		ClientCertPath:       gf.ClientCert,
		ClientKeyPath:        gf.ClientKey,
		TLSSignerName:        gf.TLSSigner,
		TLSSignerParams:      tlsSignerParams,
		CACertPath:           gf.CACert,
		TLSMinVersion:        tlsMinVersion,
		Timeout:              timeout,
		AcceptHeader:         c.content.AcceptHeader(),
		AcceptEncodingHeader: c.content.AcceptEncodingHeader(),
		ContentType:          gf.ContentType,
		Transport:            c.baseHTTPTransport(),
		CacheDir:             c.cacheDir(),
		CacheMaxBytes:        c.cacheMaxBytes(),
		NoCache:              gf.NoCache,
		Retry:                retry,
		RetryBaseDelay:       c.hooks.RetryBaseDelay,
		Logger:               c.Stderr,
	}, nil
}

func (c *CLI) cacheMaxBytes() int64 {
	if c == nil || c.cfg == nil {
		return 0
	}
	return cacheSizeStringToBytes(c.cfg.Cache.MaxSize)
}

func cacheSizeStringToBytes(s string) int64 {
	if s == "" {
		return 0
	}
	parsed, err := parseByteSize(s)
	if err != nil {
		return 0
	}
	return parsed
}

// parseByteSize parses strings like "100MB", "64MiB", "1024", and returns bytes.
func parseByteSize(s string) (int64, error) {
	v := strings.TrimSpace(strings.ToUpper(s))
	if v == "" {
		return 0, fmt.Errorf("empty size")
	}

	mult := int64(1)
	switch {
	case strings.HasSuffix(v, "GIB"):
		mult = 1024 * 1024 * 1024
		v = strings.TrimSuffix(v, "GIB")
	case strings.HasSuffix(v, "MIB"):
		mult = 1024 * 1024
		v = strings.TrimSuffix(v, "MIB")
	case strings.HasSuffix(v, "KIB"):
		mult = 1024
		v = strings.TrimSuffix(v, "KIB")
	case strings.HasSuffix(v, "GB"):
		mult = 1000 * 1000 * 1000
		v = strings.TrimSuffix(v, "GB")
	case strings.HasSuffix(v, "MB"):
		mult = 1000 * 1000
		v = strings.TrimSuffix(v, "MB")
	case strings.HasSuffix(v, "KB"):
		mult = 1000
		v = strings.TrimSuffix(v, "KB")
	case strings.HasSuffix(v, "B"):
		v = strings.TrimSuffix(v, "B")
	}

	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("size must be >= 0")
	}
	return n * mult, nil
}
