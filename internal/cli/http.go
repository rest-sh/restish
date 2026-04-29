package cli

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	urlpath "path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/content"
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
	return c.configScopedCacheDir(c.paths().Cache())
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
		use := v.name + " <url>"
		switch method {
		case "POST", "PUT", "PATCH":
			use += " [body...]"
		}
		cmd := &cobra.Command{
			Use:     use,
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
	return c.runHTTPInternal(cmd, method, args, false, nil, false, "", "")
}

// runHTTPInternal is the implementation of runHTTP. followMode=true is used for
// follow-up requests triggered by response-middleware plugins; in that mode,
// response-middleware is skipped to prevent infinite loops.
// extraHeaders holds additional "Name: Value" strings injected by generated
// commands (e.g. OpenAPI header/cookie parameters) that must not be stored on
// the cobra.Command itself since command objects are reused across invocations.
// noAuth strips authentication when following a redirect to a different host,
// preventing credentialed SSRF via a compromised response-middleware plugin.
func (c *CLI) runHTTPInternal(cmd *cobra.Command, method string, args []string, followMode bool, extraHeaders []string, noAuth bool, firstPartyHost string, contentTypeOverride string, bodySchemaTypes ...map[string]string) error {
	var opts requestBodyOptions
	if len(bodySchemaTypes) > 0 {
		opts.schemaTypes = bodySchemaTypes[0]
	}
	return c.runHTTPInternalWithBodyOptions(cmd, method, args, followMode, extraHeaders, noAuth, firstPartyHost, contentTypeOverride, opts)
}

type requestBodyOptions struct {
	schemaTypes               map[string]string
	multipartPartContentTypes map[string]string
}

func (c *CLI) runHTTPInternalWithBodyOptions(cmd *cobra.Command, method string, args []string, followMode bool, extraHeaders []string, noAuth bool, firstPartyHost string, contentTypeOverride string, bodyOpts requestBodyOptions) error {
	rawURL := args[0]
	bodyArgs := args[1:] // positional args after the URL are shorthand body input

	opts, err := c.httpOptsFromFlags(cmd)
	if err != nil {
		return err
	}
	if opts.ContentType == "" && contentTypeOverride != "" {
		opts.ContentType = contentTypeOverride
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
	bodyVal, err := input.BodyWithSchemaTypes(c.Stdin, stdinIsTTY, bodyArgs, opts.ContentType, bodyOpts.schemaTypes)
	if err != nil {
		return fmt.Errorf("building request body: %w", err)
	}
	if len(bodyOpts.multipartPartContentTypes) > 0 && strings.HasPrefix(strings.ToLower(opts.ContentType), "multipart/form-data") {
		bodyVal = content.MultipartBody{Value: bodyVal, ContentTypes: bodyOpts.multipartPartContentTypes}
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
			firstPartyHost = u.Scheme + "://" + u.Host
		}
	}

	httpResp, err := c.sendPreparedRequest(requestContext(cmd), method, prepared)
	if err != nil {
		if hint := networkErrorHint(err); hint != "" {
			return fmt.Errorf("network error for %s %s: %w\nhint: %s", method, rawURL, err, hint)
		}
		return fmt.Errorf("network error for %s %s: %w", method, rawURL, err)
	}

	// Streaming responses (SSE, NDJSON) are handled before body normalization.
	if kind := streamingContentType(httpResp.Header.Get("Content-Type")); kind != "" {
		if err := c.statusError(cmd, httpResp.StatusCode); err != nil {
			_ = httpResp.Body.Close()
			return err
		}
		var streamErr error
		switch kind {
		case "sse":
			streamErr = c.handleSSE(cmd, httpResp)
		case "ndjson":
			streamErr = c.handleNDJSON(cmd, httpResp)
		}
		if streamErr != nil {
			return streamErr
		}
		return nil
	}

	resp, err := c.normalizeHTTPResponse(httpResp, maxBodyBytes(cmd))
	if err != nil {
		return err
	}
	if v := globalFlagsFromContext(requestContext(cmd)).Verbose; v >= 1 {
		c.logVerboseResponseBody(resp)
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
				c.warnf("response-middleware follow to different host %q — stripping credentials", followReq.URI)
			}
			return c.runHTTPInternal(cmd, followReq.Method, []string{followReq.URI}, true, nil, crossHost, firstPartyHost, "")
		}
	}

	// Pagination: if this is a GET and there's a next link, paginate.
	gf := globalFlagsFromContext(requestContext(cmd))
	if method == "GET" && !gf.HeadersShorthand && !filterRequestsResponseMetadata(gf.Filter) {
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

	return c.statusError(cmd, resp.Status)
}

func (c *CLI) statusError(cmd *cobra.Command, status int) error {
	if globalFlagsFromContext(requestContext(cmd)).IgnoreStatus {
		return nil
	}
	if code := output.StatusToExitCode(status); code != 0 {
		return &ExitCodeError{Code: code}
	}
	return nil
}

func followCrossesFirstPartyHost(firstPartyOrigin, followURI string) bool {
	if firstPartyOrigin == "" {
		return false
	}
	followU, err := url.Parse(followURI)
	if err != nil {
		return false
	}
	baseU, err := url.Parse(firstPartyOrigin)
	if err != nil || baseU.Scheme == "" {
		return !strings.EqualFold(followU.Hostname(), firstPartyOrigin)
	}
	return !sameURLOrigin(baseU, followU)
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
	explicitFilter := explicitOutputFilter(gf)
	filterLang := gf.FilterLang
	headersOnly := gf.HeadersShorthand
	tty := output.IsTerminal(c.Stdout)

	if gf.Raw && !explicitFilter && fmtName == "" {
		_, err := c.Stdout.Write(resp.Raw)
		return err
	}
	if !tty && !explicitFilter && fmtName == "" && strings.HasPrefix(resp.Headers["Content-Type"], "image/") {
		_, err := c.Stdout.Write(resp.Raw)
		return err
	}

	if headersOnly && filterExpr != "" {
		c.warnf(`--rsh-headers overrides -f; using "headers"`)
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

	if explicitFilter && filterNeedsLinks(filterExpr) {
		c.ensureBodyLinks(resp)
	}

	if filterExpr == "@" && !explicitFilter {
		formatter, err := c.selectFormatter(cmd, fmtName, tty)
		if err != nil {
			return err
		}
		return formatter.Format(c.Stdout, resp, output.ColorEnabled(c.Stdout))
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
		c.hintf("filter returned no results; to access response body fields use 'body.%s'", filterExpr)
	}

	if handled {
		return nil
	}

	if explicitFilter && filterExpr == "@" {
		return c.renderValue(cmd, doc)
	}

	if filterExpr != "@" {
		return c.renderValue(cmd, filtered)
	}

	formatter, err := c.selectFormatter(cmd, fmtName, tty)
	if err != nil {
		return err
	}

	return formatter.Format(c.Stdout, resp, output.ColorEnabled(c.Stdout))
}

func explicitOutputFilter(gf GlobalFlags) bool {
	return gf.Filter != "" || gf.HeadersShorthand
}

func filterNeedsLinks(filterExpr string) bool {
	filterExpr = strings.TrimSpace(filterExpr)
	return filterExpr == "@" || filterExpr == "links" ||
		strings.HasPrefix(filterExpr, "links.") ||
		strings.HasPrefix(filterExpr, "links[") ||
		strings.HasPrefix(filterExpr, ".links")
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

	formatter, err := c.selectFormatter(cmd, fmtName, tty)
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

func (c *CLI) selectFormatter(cmd *cobra.Command, fmtName string, tty bool) (output.Formatter, error) {
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
		formatter, ok = output.SelectDefault(fmts, tty)
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
	if strings.HasPrefix(filterExpr, ".") || strings.ContainsAny(filterExpr, "|()") {
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

func filterRequestsResponseMetadata(expr string) bool {
	expr = strings.TrimSpace(expr)
	return expr == "@" ||
		expr == "headers" || strings.HasPrefix(expr, "headers.") || strings.HasPrefix(expr, "headers[") ||
		expr == "status" || strings.HasPrefix(expr, "status.") || strings.HasPrefix(expr, "status[") ||
		expr == "proto" || strings.HasPrefix(expr, "proto.") || strings.HasPrefix(expr, "proto[")
}

// logVerbose prints request and response summary lines to stderr.
// Sensitive request headers (Authorization, Cookie, Set-Cookie,
// Proxy-Authorization) are redacted to avoid leaking credentials.
// At verbose >= 2 it also dumps TLS version, cipher suite, and peer
// certificate chain (subject, issuer, expiry).
func (c *CLI) logVerbose(resp *http.Response, verbose int) {
	req := resp.Request
	if req == nil {
		return
	}
	fmt.Fprintf(c.Stderr, "> %s %s\n", req.Method, redactedRequestURL(req.URL))
	for k, vs := range req.Header {
		for _, v := range vs {
			if isSensitiveHeader(k) {
				v = "<redacted>"
			}
			fmt.Fprintf(c.Stderr, "> %s: %s\n", k, v)
		}
	}
	c.logVerboseRequestBody(req)
	fmt.Fprintln(c.Stderr, ">")
	if resp.Header.Get("X-From-Cache") != "" {
		fmt.Fprintln(c.Stderr, "* Cache: HIT")
	}
	fmt.Fprintf(c.Stderr, "< %s %d %s\n", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode))
	for k, vs := range resp.Header {
		for _, v := range vs {
			if isSensitiveHeader(k) {
				v = "<redacted>"
			}
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

const verboseBodyLimit = 4096

func (c *CLI) logVerboseRequestBody(req *http.Request) {
	if req.GetBody != nil {
		if body, err := req.GetBody(); err == nil {
			data, _ := io.ReadAll(io.LimitReader(body, verboseBodyLimit+1))
			_ = body.Close()
			c.logVerboseBody("> body", data, req.Header.Get("Content-Type"))
		}
	}
}

func (c *CLI) logVerboseResponseBody(resp *output.Response) {
	if resp != nil && len(resp.Raw) > 0 {
		c.logVerboseBody("< body", resp.Raw, resp.Headers["Content-Type"])
	}
}

func (c *CLI) logVerboseBody(label string, data []byte, contentType string) {
	if len(data) == 0 {
		return
	}
	truncated := len(data) > verboseBodyLimit
	if truncated {
		data = data[:verboseBodyLimit]
	}
	rendered := redactVerboseBody(data, contentType)
	if rendered == "" {
		return
	}
	fmt.Fprintf(c.Stderr, "%s:\n%s\n", label, rendered)
	if truncated {
		fmt.Fprintf(c.Stderr, "%s truncated after %d bytes\n", label, verboseBodyLimit)
	}
}

func redactVerboseBody(data []byte, contentType string) string {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if mediaType == "application/json" || strings.HasSuffix(mediaType, "+json") {
		var value any
		if err := json.Unmarshal(data, &value); err == nil {
			redactSensitiveJSON(value)
			if out, err := json.MarshalIndent(value, "", "  "); err == nil {
				return string(out)
			}
		}
	}
	if !json.Valid(data) && strings.ContainsRune(string(data), '\x00') {
		return ""
	}
	return string(data)
}

func redactSensitiveJSON(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if isSensitiveQueryParam(key) {
				v[key] = "<redacted>"
				continue
			}
			redactSensitiveJSON(item)
		}
	case []any:
		for _, item := range v {
			redactSensitiveJSON(item)
		}
	}
}

func networkErrorHint(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "certificate") || strings.Contains(msg, "x509:"):
		return "check the server certificate, use --rsh-ca-cert for a private CA, or --rsh-insecure only for deliberate testing"
	case strings.Contains(msg, "no such host"):
		return "check the hostname, DNS, VPN, or proxy settings"
	case strings.Contains(msg, "connection refused"):
		return "check that the service is running and that the host and port are correct"
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded"):
		return "check network reachability or increase --rsh-timeout"
	default:
		return ""
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
	lower := strings.ToLower(name)
	for _, marker := range []string{"api-key", "apikey", "auth-token", "token", "secret", "password"} {
		if strings.Contains(lower, marker) {
			return true
		}
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

	match, ok, err := c.matchAPIProfile(rawURL, profileName)
	if err != nil {
		return rawURL, "", opts, err
	}
	if !ok {
		return rawURL, "", opts, nil
	}
	rawURL = match.rawURL

	if match.profile == nil {
		if match.api.Profiles != nil {
			return rawURL, match.apiName, opts, fmt.Errorf("profile %q not found for API %q; configured profiles: %s", profileName, match.apiName, profileNames(match.api.Profiles))
		}
		callbacks := c.authOnRequest(match.apiName, profileName, nil, authOpts)
		opts.OnRequest = callbacks.OnRequest
		opts.OnUnauthorized = callbacks.OnUnauthorized
	}

	// Prepend persistent profile headers/query so flag-supplied values take
	// precedence (they appear later in the slice, and are applied last).
	if match.profile != nil {
		opts.Headers = append(append([]string(nil), match.profile.Headers...), opts.Headers...)
		opts.Query = append(append([]string(nil), match.profile.Query...), opts.Query...)
		callbacks := c.authOnRequest(match.apiName, profileName, match.profile, authOpts)
		opts.OnRequest = callbacks.OnRequest
		opts.OnUnauthorized = callbacks.OnUnauthorized
		if opts.TLSSignerName == "" {
			opts.TLSSignerName = match.profile.TLSSigner
		}
		opts.TLSSignerParams = mergeTLSSignerParams(opts.TLSSignerParams, match.profile.TLSSignerParams)
	}
	if match.apiName != "" {
		opts.CacheNamespace = match.apiName + ":" + profileName
	}

	return rawURL, match.apiName, opts, nil
}

type apiProfileMatch struct {
	apiName string
	api     *config.APIConfig
	profile *config.ProfileConfig
	rawURL  string
	score   int
}

func (c *CLI) matchAPIProfile(rawURL, profileName string) (apiProfileMatch, bool, error) {
	apiName, rest, _ := strings.Cut(rawURL, "/")
	if api := c.cfg.APIs[apiName]; api != nil {
		baseURL := api.BaseURL
		prof := profileForName(api, profileName)
		if prof != nil && prof.BaseURL != "" {
			baseURL = prof.BaseURL
		}
		expanded := strings.TrimRight(baseURL, "/")
		if rest != "" {
			expanded += "/" + rest
		}
		expanded = cleanExpandedAPIURL(expanded)
		return apiProfileMatch{apiName: apiName, api: api, profile: prof, rawURL: expanded, score: len(apiName)}, true, nil
	}

	var best apiProfileMatch
	var ties []string
	for name, api := range c.cfg.APIs {
		if api == nil {
			continue
		}
		prof := profileForName(api, profileName)
		bases, err := apiMatchBases(api, prof)
		if err != nil {
			c.warnf("API %q: %v", name, err)
		}
		for _, base := range bases {
			score, ok := matchURLBase(rawURL, base)
			if !ok || score < best.score {
				continue
			}
			if score == best.score && best.apiName != "" {
				ties = append(ties, name)
				continue
			}
			best = apiProfileMatch{apiName: name, api: api, profile: prof, rawURL: rawURL, score: score}
			ties = []string{name}
		}
	}
	if best.apiName != "" && len(ties) > 1 {
		sort.Strings(ties)
		return apiProfileMatch{}, false, fmt.Errorf("ambiguous API match for %s: %s all match with the same base URL score; use the API short-name form instead", rawURL, strings.Join(ties, ", "))
	}
	return best, best.apiName != "", nil
}

func profileForName(api *config.APIConfig, profileName string) *config.ProfileConfig {
	if api == nil || api.Profiles == nil {
		return nil
	}
	return api.Profiles[profileName]
}

func cleanExpandedAPIURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if u.Path == "" {
		return rawURL
	}
	cleaned := urlpath.Clean(u.Path)
	if strings.HasSuffix(u.Path, "/") && cleaned != "/" {
		cleaned += "/"
	}
	u.Path = cleaned
	return u.String()
}

func apiMatchBases(api *config.APIConfig, prof *config.ProfileConfig) ([]string, error) {
	var bases []string
	if api.BaseURL != "" {
		bases = append(bases, api.BaseURL)
	}
	if prof != nil && prof.BaseURL != "" {
		bases = append(bases, prof.BaseURL)
	}
	if api.OperationBase != "" {
		resolved, err := config.ResolveOperationBaseURL(api.BaseURL, api.OperationBase)
		if err != nil {
			return bases, fmt.Errorf("operation_base: %w", err)
		}
		bases = append(bases, resolved)
	}
	return bases, nil
}

func matchURLBase(rawURL, rawBase string) (int, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || !u.IsAbs() {
		return 0, false
	}
	base, err := url.Parse(rawBase)
	if err != nil || !base.IsAbs() {
		return 0, false
	}
	if !sameURLOrigin(base, u) {
		return 0, false
	}
	basePath := strings.TrimRight(base.EscapedPath(), "/")
	if basePath == "" {
		basePath = "/"
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if basePath != "/" {
		if path != basePath && !strings.HasPrefix(path, basePath+"/") {
			return 0, false
		}
	}
	return len(base.Scheme) + len(base.Host) + len(basePath), true
}

func sameURLOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) &&
		effectivePort(a) == effectivePort(b)
}

func effectivePort(u *url.URL) string {
	if u == nil {
		return ""
	}
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	}
	if _, port, err := net.SplitHostPort(u.Host); err == nil {
		return port
	}
	return ""
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
		c.warnf("TLS certificate verification is disabled (--rsh-insecure); connections are not secure")
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
	cacheMaxBytes, err := c.cacheMaxBytes()
	if err != nil {
		return request.Options{}, err
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
		UserAgent:            "restish/" + Version,
		Transport:            c.baseHTTPTransport(),
		CacheDir:             c.cacheDir(),
		CacheMaxBytes:        cacheMaxBytes,
		NoCache:              gf.NoCache,
		Retry:                retry,
		RetryBaseDelay:       c.hooks.RetryBaseDelay,
		Logger:               diagnosticPrefixWriter(c.Stderr),
		WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
			if gf.Verbose < 1 {
				return rt
			}
			return &verboseTransport{inner: rt, cli: c, verbose: gf.Verbose}
		},
	}, nil
}

func (c *CLI) cacheMaxBytes() (int64, error) {
	if c == nil || c.cfg == nil {
		return 0, nil
	}
	return cacheSizeStringToBytes(c.cfg.Cache.MaxSize)
}

func cacheSizeStringToBytes(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	parsed, err := parseByteSize(s)
	if err != nil {
		return 0, fmt.Errorf("invalid cache.max_size %q: %w", s, err)
	}
	return parsed, nil
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
