package cli

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	urlpath "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/content"
	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/rest-sh/restish/v2/internal/input"
	"github.com/rest-sh/restish/v2/internal/output"
	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/rest-sh/restish/v2/internal/secrets"
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
			Annotations: map[string]string{
				requestHelpAnnotation: "true",
			},
			Args:              cobra.MinimumNArgs(1),
			ValidArgsFunction: c.completeHTTPURL(method),
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
	return c.runHTTPWithOptions(cmd, method, args, false, nil, false, "", "", requestBodyOptions{})
}

type requestBodyOptions struct {
	schemaTypes               map[string]string
	multipartPartContentTypes map[string]string
	operationAuth             *operationAuthPolicy
	bodyRequired              bool
	bodyOverrideSet           bool
	bodyOverride              any
}

// runHTTPWithOptions executes one HTTP request through the full pipeline:
// auth, request middleware, retries, streaming, response middleware,
// pagination, and formatting.
//
// followMode=true is used for follow-up requests triggered by
// response-middleware plugins; in that mode, response-middleware is skipped to
// prevent infinite loops. extraHeaders holds additional "Name: Value" strings
// injected by generated commands (e.g. OpenAPI header/cookie parameters) that
// must not be stored on the cobra.Command itself since command objects are
// reused across invocations. noAuth strips authentication when following a
// redirect to a different host, preventing credentialed SSRF via a compromised
// response-middleware plugin.
func (c *CLI) runHTTPWithOptions(cmd *cobra.Command, method string, args []string, followMode bool, extraHeaders []string, noAuth bool, firstPartyHost string, contentTypeOverride string, bodyOpts requestBodyOptions) error {
	trace := ensureRequestTrace(cmd)
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

	var bodyVal any
	var bodyInfo input.BodyInfo
	if bodyOpts.bodyOverrideSet {
		bodyVal = bodyOpts.bodyOverride
	} else {
		// Build request body from shorthand args and/or piped stdin.
		stdinIsTTY := output.IsTerminalReader(c.Stdin)
		var err error
		bodyVal, bodyInfo, err = input.BodyWithInfo(c.Stdin, stdinIsTTY, bodyArgs, opts.ContentType, input.BodyOptions{
			SchemaTypes: bodyOpts.schemaTypes,
			Warnf:       c.warnf,
		})
		if err != nil {
			return fmt.Errorf("building request body: %w", err)
		}
	}
	if bodyOpts.bodyRequired && bodyVal == nil {
		return fmt.Errorf("request body is required; pass body arguments, pipe a body on stdin, or run %q for an example", cmd.CommandPath()+" --rsh-generate-body")
	}
	if len(bodyOpts.multipartPartContentTypes) > 0 && strings.HasPrefix(strings.ToLower(opts.ContentType), "multipart/form-data") {
		bodyVal = content.MultipartBody{Value: bodyVal, ContentTypes: bodyOpts.multipartPartContentTypes}
	}
	inputSource := traceInputSource(bodyInfo, bodyVal != nil)
	if bodyVal != nil {
		trace.Step(inputSource)
	}

	prepared, err := c.prepareRequest(requestContext(cmd), rawURL, profileName, opts, bodyVal, extraHeaders, noAuth, authOpts, bodyOpts.operationAuth)
	if err != nil {
		return err
	}
	defer c.closePreparedTransport(prepared)
	rawURL = prepared.rawURL
	apiName = prepared.apiName
	opts = prepared.opts
	c.populateRequestTrace(trace, apiName, profileName, inputSource, prepared)
	trace.RenderBefore(c.Stderr, globalFlagsFromContext(requestContext(cmd)).Verbose)
	c.warnRetryUnsafe(method, opts)
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
	trace.Step("HTTP")

	// Streaming responses (SSE, NDJSON) are handled before body normalization.
	if kind := streamingContentType(httpResp.Header.Get("Content-Type")); kind != "" {
		traceContentDecode(trace, httpResp.Header.Get("Content-Type"))
		if gf := globalFlagsFromContext(requestContext(cmd)); gf.Raw {
			if err := validateRawOutputOptions(gf); err != nil {
				_ = httpResp.Body.Close()
				return err
			}
			defer httpResp.Body.Close()
			if err := c.statusError(cmd, httpResp.StatusCode); err != nil {
				return err
			}
			trace.Info("Output", "raw")
			trace.Step("raw")
			trace.RenderAfter(c.Stderr, gf.Verbose)
			_, err := io.Copy(c.Stdout, httpResp.Body)
			return err
		}
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
	traceContentDecode(trace, output.Header(resp.Headers, "Content-Type"))
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
			followHeaders, followContentType := followRequestHeaders(followReq)
			if followReq.ContentType != "" {
				followContentType = followReq.ContentType
			}
			return c.runHTTPWithOptions(cmd, followReq.Method, []string{followReq.URI}, true, followHeaders, crossHost, firstPartyHost, followContentType, requestBodyOptions{
				bodyOverrideSet: true,
				bodyOverride:    followReq.Body,
			})
		}
	}

	// Pagination: if this is a GET and there's a next link, paginate.
	gf := globalFlagsFromContext(requestContext(cmd))
	if method == "GET" && !gf.Raw && !gf.HeadersShorthand && !filterRequestsResponseMetadata(gf.Filter) {
		var pagCfg *config.PaginationConfig
		if apiName != "" && c.cfg != nil && c.cfg.APIs[apiName] != nil {
			pagCfg = c.cfg.APIs[apiName].Pagination
		}
		did, err := c.tryPaginate(cmd, resp, rawURL, opts, pagCfg)
		if err != nil {
			return err
		}
		if did {
			return nil
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
	return !request.SameOrigin(baseU, followU)
}

func followRequestHeaders(followReq *HookFollowRequest) ([]string, string) {
	if followReq == nil || len(followReq.Headers) == 0 {
		return nil, ""
	}
	headers := make([]string, 0, len(followReq.Headers))
	var contentType string
	for name, value := range followReq.Headers {
		if strings.EqualFold(name, "Content-Type") && followReq.Body != nil {
			contentType = value
			continue
		}
		headers = append(headers, name+": "+value)
	}
	return headers, contentType
}

// formatResponse applies any filter then selects and runs the formatter.
func (c *CLI) formatResponse(cmd *cobra.Command, resp *output.Response) error {
	// Silent mode: suppress all output.
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.Silent {
		return nil
	}
	if err := validateRawOutputOptions(gf); err != nil {
		return err
	}

	fmtName := gf.OutputFormat
	filterExpr := gf.Filter
	explicitFilter := explicitOutputFilter(gf)
	filterLang := gf.FilterLang
	headersOnly := gf.HeadersShorthand
	statusOnly := gf.StatusShorthand
	tty := output.IsTerminal(c.Stdout)

	if gf.Raw {
		trace := requestTraceFromContext(requestContext(cmd))
		trace.Info("Output", "raw")
		trace.Step("raw")
		trace.RenderAfter(c.Stderr, gf.Verbose)
		return c.writeRawBytes(resp.Raw)
	}
	if !tty && !explicitFilter && fmtName == "" && defaultRawBytesResponse(resp) {
		trace := requestTraceFromContext(requestContext(cmd))
		trace.Info("Output", "raw bytes (auto)")
		trace.Step("raw")
		trace.RenderAfter(c.Stderr, gf.Verbose)
		_, err := c.Stdout.Write(resp.Raw)
		return err
	}

	if headersOnly && filterExpr != "" {
		c.warnf(`--rsh-headers overrides -f; using "headers"`)
	}
	if statusOnly && filterExpr != "" {
		c.warnf(`--rsh-status overrides -f; using "status"`)
	}
	if headersOnly && statusOnly {
		c.warnf(`--rsh-status overrides --rsh-headers; using "status"`)
		headersOnly = false
	}
	if headersOnly {
		filterExpr = "headers"
	}
	if statusOnly {
		filterExpr = "status"
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
		traceOutputFormatter(cmd, fmtName, tty, formatter)
		if trace := requestTraceFromContext(requestContext(cmd)); trace != nil {
			trace.RenderAfter(c.Stderr, gf.Verbose)
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
		"proto":       resp.Proto,
		"status":      resp.Status,
		"headers":     firstHeaderValues(resp.Headers),
		"headers_all": resp.Headers,
		"links":       resp.Links,
		"body":        resp.Body,
	}

	filterResult, err := filter.ApplyWithInfo(filterExpr, doc, lang)
	if err != nil {
		return fmt.Errorf("filter: %w", err)
	}
	filtered := filterResult.Value
	if explicitFilter && gf.Verbose >= 1 {
		trace := requestTraceFromContext(requestContext(cmd))
		traceFilter(trace, lang, filterResult.Lang)
	}

	if filtered == nil && shouldSuggestBodyPrefix(filterExpr) {
		c.hintf("filter returned no results; to access response body fields use 'body.%s'", filterExpr)
	}

	if explicitFilter && filterExpr == "@" {
		return c.renderValue(cmd, doc, true)
	}

	if filterExpr != "@" {
		return c.renderValue(cmd, filtered, explicitFilter)
	}

	formatter, err := c.selectFormatter(cmd, fmtName, tty)
	if err != nil {
		return err
	}

	traceOutputFormatter(cmd, fmtName, tty, formatter)
	if trace := requestTraceFromContext(requestContext(cmd)); trace != nil {
		trace.RenderAfter(c.Stderr, gf.Verbose)
	}
	return formatter.Format(c.Stdout, resp, output.ColorEnabled(c.Stdout))
}

func firstHeaderValues(headers map[string][]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, values := range headers {
		if len(values) > 0 {
			out[k] = values[0]
		}
	}
	return out
}

func (c *CLI) populateRequestTrace(trace *requestTrace, apiName, profileName, inputSource string, prepared *preparedRequest) {
	if trace == nil || prepared == nil {
		return
	}
	trace.InfoBefore("Config", c.configFilePath())
	if apiName != "" {
		trace.InfoBefore("API", apiName)
	}
	trace.InfoBefore("Profile", profileName)
	if prepared.authEnabled {
		trace.InfoBefore("Auth", "enabled")
	} else {
		trace.InfoBefore("Auth", "none")
	}
	trace.InfoBefore("Input", inputSource)
	if prepared.bodyContentType != "" {
		trace.InfoBefore("Request body", traceMediaType(prepared.bodyContentType))
		trace.Step(traceMediaType(prepared.bodyContentType))
	} else {
		trace.InfoBefore("Request body", "none")
	}
	if prepared.authEnabled {
		trace.Step("auth")
	}
	if len(c.pluginsByHook["request-middleware"]) > 0 {
		trace.DebugBefore("Request plugins", pluginNameList(c.pluginsByHook["request-middleware"]))
	}
}

func defaultRawBytesResponse(resp *output.Response) bool {
	if resp == nil || len(resp.Raw) == 0 {
		return false
	}
	contentType := output.Header(resp.Headers, "Content-Type")
	base, _, _ := mime.ParseMediaType(contentType)
	switch {
	case strings.HasPrefix(base, "image/"):
		return true
	case base == "application/octet-stream" || base == "application/zip":
		return true
	}
	_, bodyIsBytes := resp.Body.([]byte)
	return bodyIsBytes
}

func traceInputSource(info input.BodyInfo, hasBody bool) string {
	switch {
	case !hasBody:
		return "none"
	case info.UsedStdin && info.UsedArgs:
		return "stdin + args"
	case info.UsedStdin:
		return "stdin"
	case info.UsedArgs:
		return "args"
	default:
		return "none"
	}
}

func traceContentDecode(trace *requestTrace, contentType string) {
	mediaType := traceMediaType(contentType)
	if mediaType == "" {
		mediaType = "identity"
	}
	trace.Info("Decode", mediaType)
	trace.Step(mediaType)
}

func traceFilter(trace *requestTrace, requested, resolved filter.Lang) {
	if trace == nil {
		return
	}
	if requested == filter.LangAuto {
		value := fmt.Sprintf("%s (auto)", resolved)
		trace.Info("Filter", value)
		trace.Step(filterPipelineStep(resolved, true))
		return
	}
	trace.Info("Filter", requested.String())
	trace.Step(filterPipelineStep(requested, false))
}

func filterPipelineStep(lang filter.Lang, auto bool) string {
	if auto {
		return fmt.Sprintf("%s(auto)", lang)
	}
	return lang.String()
}

func traceMediaType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		return mediaType
	}
	return strings.Split(contentType, ";")[0]
}

func pluginNameList(plugins []internalplugin.Plugin) string {
	names := make([]string, 0, len(plugins))
	for _, p := range plugins {
		if p.Manifest.Name != "" {
			names = append(names, p.Manifest.Name)
			continue
		}
		names = append(names, filepath.Base(p.Path))
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func traceOutputFormatter(cmd *cobra.Command, fmtName string, tty bool, formatter output.Formatter) {
	trace := requestTraceFromContext(requestContext(cmd))
	if trace == nil {
		return
	}
	name := resolvedOutputFormatName(fmtName, tty)
	value := outputTraceLabel(name, formatter)
	if fmtName == "" {
		value += " (auto)"
	}
	trace.Info("Output", value)
	if _, ok := formatter.(*output.PluginFormatter); ok {
		trace.AddInfo("Plugin", "formatter "+name)
	}
	trace.Step(outputPipelineStep(name, formatter))
}

func (c *CLI) traceValueOutput(cmd *cobra.Command, value any, plainScalars bool) {
	trace := requestTraceFromContext(requestContext(cmd))
	if trace == nil {
		return
	}
	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.Silent {
		trace.Info("Output", "silent")
		trace.Step("silent")
		return
	}
	if gf.Raw {
		trace.Info("Output", "raw")
		trace.Step("raw")
		return
	}

	fmtName := gf.OutputFormat
	if fmtName == "" && plainScalars && output.IsLineScalar(value) {
		trace.Info("Output", "lines (auto)")
		trace.Step("lines")
		return
	}
	if fmtName == "" {
		if output.IsTerminal(c.Stdout) {
			trace.Info("Output", "readable (auto)")
			trace.Step("readable")
			return
		}
		trace.Info("Output", "json (auto)")
		trace.Step("json")
		return
	}
	if formatter := c.formatters[fmtName]; formatter != nil {
		trace.Info("Output", outputTraceLabel(fmtName, formatter))
		if _, ok := formatter.(*output.PluginFormatter); ok {
			trace.AddInfo("Plugin", "formatter "+fmtName)
		}
		trace.Step(outputPipelineStep(fmtName, formatter))
		return
	}
	trace.Info("Output", fmtName)
	trace.Step(fmtName)
}

func resolvedOutputFormatName(fmtName string, tty bool) string {
	if fmtName != "" {
		return fmtName
	}
	if tty {
		return "readable"
	}
	return "json"
}

func outputTraceLabel(name string, formatter output.Formatter) string {
	if _, ok := formatter.(*output.PluginFormatter); ok {
		return name + " plugin"
	}
	return name
}

func outputPipelineStep(name string, formatter output.Formatter) string {
	if _, ok := formatter.(*output.PluginFormatter); ok {
		return name + "(plugin)"
	}
	return name
}

func explicitOutputFilter(gf GlobalFlags) bool {
	return gf.Filter != "" || gf.HeadersShorthand || gf.StatusShorthand
}

func validateRawOutputOptions(gf GlobalFlags) error {
	if !gf.Raw {
		return nil
	}
	if explicitOutputFilter(gf) {
		return fmt.Errorf("--rsh-raw cannot be combined with --rsh-filter, --rsh-headers, or --rsh-status\nFor shell-friendly scalar output use: -o lines\nFor JSON use: -o json")
	}
	if gf.OutputFormat != "" {
		return fmt.Errorf("--rsh-raw cannot be combined with --rsh-output-format; raw output writes the response body bytes")
	}
	return nil
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
func (c *CLI) renderValue(cmd *cobra.Command, value any, plainScalars bool) error {
	renderer, err := c.newValueRenderer(cmd, nil, plainScalars)
	if err != nil {
		return err
	}
	defer renderer.Close()
	c.traceValueOutput(cmd, value, plainScalars)
	if trace := requestTraceFromContext(requestContext(cmd)); trace != nil {
		trace.RenderAfter(c.Stderr, globalFlagsFromContext(requestContext(cmd)).Verbose)
	}
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

func (c *CLI) newValueRenderer(cmd *cobra.Command, base *output.Response, plainScalars bool) (valueRenderer, error) {
	if base == nil {
		base = &output.Response{}
	}

	gf := globalFlagsFromContext(requestContext(cmd))
	if gf.Silent {
		return valueRendererFunc{render: func(any) error { return nil }}, nil
	}

	if gf.Raw {
		return nil, validateRawOutputOptions(gf)
	}

	fmtName := gf.OutputFormat
	tty := output.IsTerminal(c.Stdout)
	color := output.ColorEnabled(c.Stdout)

	if fmtName == "" && plainScalars {
		return valueRendererFunc{render: func(value any) error {
			if output.IsLineScalar(value) {
				return output.WriteLinesValue(c.Stdout, value)
			}
			return c.renderValueWithDefaults(value, tty, color)
		}}, nil
	}

	// Body/sub-value rendering should stay machine-friendly by default on
	// non-TTY.
	if !tty && fmtName == "" {
		return valueRendererFunc{render: func(value any) error { return c.writeJSONValue(value) }}, nil
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

func (c *CLI) renderValueWithDefaults(value any, tty, color bool) error {
	if !tty {
		return c.writeJSONValue(value)
	}
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
}

func (c *CLI) writeJSONValue(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = c.Stdout.Write(encoded)
	return err
}

func (c *CLI) selectFormatter(cmd *cobra.Command, fmtName string, tty bool) (output.Formatter, error) {
	if fmtName == "raw" {
		return nil, fmt.Errorf(`output format "raw" has been removed; use -r/--rsh-raw for raw response body bytes or -o lines for shell-friendly text`)
	}

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

	if fmtName == "" {
		if tty {
			return fmts["readable"], nil
		}
		return fmts["json"], nil
	}
	formatter, ok := fmts[fmtName]
	if !ok {
		return nil, fmt.Errorf("unknown output format %q; available: %s", fmtName, output.FormatterNames(fmts))
	}
	return formatter, nil
}

func (c *CLI) writeRawBytes(data []byte) error {
	_, err := c.Stdout.Write(data)
	return err
}

func (c *CLI) writePlainValue(value any) error {
	return output.WriteLinesValue(c.Stdout, value)
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

// logVerboseRequest prints request summary lines to stderr.
// Sensitive request headers (Authorization, Cookie, Set-Cookie,
// Proxy-Authorization) are redacted to avoid leaking credentials.
func (c *CLI) logVerboseRequest(req *http.Request) {
	if req == nil {
		return
	}
	fmt.Fprintf(c.Stderr, "> %s %s\n", req.Method, redactedRequestURL(req.URL))
	for _, k := range sortedHeaderKeys(req.Header) {
		vs := req.Header[k]
		for _, v := range vs {
			if isSensitiveHeader(k) {
				v = "<redacted>"
			}
			fmt.Fprintf(c.Stderr, "> %s: %s\n", k, v)
		}
	}
	c.logVerboseRequestBody(req)
	fmt.Fprintln(c.Stderr, ">")
}

// logVerboseResponse prints response summary lines to stderr. At verbose >= 2
// it also dumps TLS version, cipher suite, and peer certificate chain (subject,
// issuer, expiry).
func (c *CLI) logVerboseResponse(resp *http.Response, verbose int) {
	if resp == nil {
		return
	}
	if resp.Header.Get("X-From-Cache") != "" {
		fmt.Fprintln(c.Stderr, "* Cache: HIT")
	}
	fmt.Fprintf(c.Stderr, "< %s %d %s\n", resp.Proto, resp.StatusCode, http.StatusText(resp.StatusCode))
	for _, k := range sortedHeaderKeys(resp.Header) {
		vs := resp.Header[k]
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

func sortedHeaderKeys(headers http.Header) []string {
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
		c.logVerboseBody("< body", resp.Raw, output.Header(resp.Headers, "Content-Type"))
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
	if mediaType == "application/x-www-form-urlencoded" {
		values, err := url.ParseQuery(string(data))
		if err == nil {
			for key := range values {
				if secrets.IsQueryParamName(key) {
					values[key] = []string{"<redacted>"}
				}
			}
			return values.Encode()
		}
	}
	if mediaType != "" && mediaType != "text/plain" && !strings.HasPrefix(mediaType, "text/") && !json.Valid(data) {
		return fmt.Sprintf("<%d bytes of %s body>", len(data), mediaType)
	}
	if !json.Valid(data) && strings.ContainsRune(string(data), '\x00') {
		if mediaType == "" {
			return fmt.Sprintf("<%d bytes of binary body>", len(data))
		}
		return fmt.Sprintf("<%d bytes of %s body>", len(data), mediaType)
	}
	return string(data)
}

func redactSensitiveJSON(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if secrets.IsJSONBodyKey(key) || secrets.IsHeaderName(key) {
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
	return request.RedactedURL(u)
}

func isSensitiveQueryParam(name string) bool {
	return secrets.IsQueryParamName(name)
}

// isSensitiveHeader reports whether a header name carries credentials and
// should be redacted in verbose output.
func isSensitiveHeader(name string) bool {
	return secrets.IsHeaderName(name)
}

// isAPIShortName reports whether arg begins with a registered API name and an
// API-name delimiter or exactly matches a registered API name.
func (c *CLI) isAPIShortName(arg string) bool {
	if c.cfg == nil {
		return false
	}
	apiName, _ := splitAPIShortNameSuffix(arg)
	return c.cfg.APIs[apiName] != nil
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

	if opts.RetryMaxWait == 0 && strings.TrimSpace(match.api.RetryMaxWait) != "" {
		retryMaxWait, parseErr := time.ParseDuration(match.api.RetryMaxWait)
		if parseErr != nil || retryMaxWait <= 0 {
			if parseErr == nil {
				parseErr = fmt.Errorf("must be greater than 0")
			}
			return rawURL, match.apiName, opts, fmt.Errorf("invalid retry_max_wait for API %q: %w", match.apiName, parseErr)
		}
		opts.RetryMaxWait = retryMaxWait
	}

	if match.profile == nil {
		if match.api.Profiles != nil || profileName != "default" {
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
		if opts.CACertPath == "" {
			opts.CACertPath = match.profile.CACertPath
		}
		if opts.ClientCertPath == "" {
			opts.ClientCertPath = match.profile.ClientCertPath
		}
		if opts.ClientKeyPath == "" {
			opts.ClientKeyPath = match.profile.ClientKeyPath
		}
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
	apiName, suffix := splitAPIShortNameSuffix(rawURL)
	if api := c.cfg.APIs[apiName]; api != nil {
		baseURL := api.BaseURL
		prof := profileForName(api, profileName)
		if prof != nil && prof.BaseURL != "" {
			baseURL = prof.BaseURL
		}
		expanded := strings.TrimRight(baseURL, "/")
		expanded += suffix
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

func splitAPIShortNameSuffix(raw string) (string, string) {
	idx := strings.IndexAny(raw, "/?#")
	if idx < 0 {
		return raw, ""
	}
	return raw[:idx], raw[idx:]
}

func profileForName(api *config.APIConfig, profileName string) *config.ProfileConfig {
	if api == nil || api.Profiles == nil {
		return nil
	}
	return api.Profiles[profileName]
}

func effectiveProfileBaseURL(api *config.APIConfig, profileName string) string {
	if api == nil {
		return ""
	}
	if prof := profileForName(api, profileName); prof != nil && prof.BaseURL != "" {
		return prof.BaseURL
	}
	return api.BaseURL
}

func effectiveOperationBase(api *config.APIConfig, profileName string) string {
	if api == nil {
		return ""
	}
	if prof := profileForName(api, profileName); prof != nil && prof.OperationBase != "" {
		return prof.OperationBase
	}
	return api.OperationBase
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
	operationBase := effectiveOperationBase(api, "")
	if prof != nil && prof.OperationBase != "" {
		operationBase = prof.OperationBase
	}
	baseURL := api.BaseURL
	if prof != nil && prof.BaseURL != "" {
		baseURL = prof.BaseURL
	}
	if operationBase != "" {
		resolved, err := config.ResolveOperationBaseURL(baseURL, operationBase)
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
	return request.SameOrigin(a, b)
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

func (c *CLI) warnRetryUnsafe(method string, opts request.Options) {
	if c == nil || c.retryUnsafeWarned || !opts.RetryUnsafe || opts.Retry <= 0 {
		return
	}
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		return
	}
	c.retryUnsafeWarned = true
	c.warnf("retrying unsafe HTTP methods can repeat side effects; --rsh-retry-unsafe is enabled for POST, PUT, PATCH, or DELETE")
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
	var retryMaxWait time.Duration
	if gf.RetryMaxWait != "" {
		retryMaxWait, err = time.ParseDuration(gf.RetryMaxWait)
		if err != nil || retryMaxWait <= 0 {
			if err == nil {
				err = fmt.Errorf("must be greater than 0")
			}
			return request.Options{}, fmt.Errorf("invalid retry max wait %q: %w", gf.RetryMaxWait, err)
		}
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
		RetryUnsafe:          gf.RetryUnsafe,
		RetryBaseDelay:       c.hooks.RetryBaseDelay,
		RetryMaxWait:         retryMaxWait,
		Logger:               diagnosticPrefixWriter(c.Stderr),
		OnBeforeRequest: func(req *http.Request) {
			if gf.Verbose > 0 {
				c.logVerboseRequest(req)
			}
		},
		OnResponse: func(resp *http.Response) {
			if gf.Verbose > 0 {
				c.logVerboseResponse(resp, gf.Verbose)
			}
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
	if strings.HasPrefix(v, "-") {
		return 0, fmt.Errorf("size must not be negative")
	}

	mult := int64(1)
	switch {
	case strings.HasSuffix(v, "TIB"):
		mult = 1024 * 1024 * 1024 * 1024
		v = strings.TrimSuffix(v, "TIB")
	case strings.HasSuffix(v, "GIB"):
		mult = 1024 * 1024 * 1024
		v = strings.TrimSuffix(v, "GIB")
	case strings.HasSuffix(v, "MIB"):
		mult = 1024 * 1024
		v = strings.TrimSuffix(v, "MIB")
	case strings.HasSuffix(v, "KIB"):
		mult = 1024
		v = strings.TrimSuffix(v, "KIB")
	case strings.HasSuffix(v, "TB"):
		mult = 1000 * 1000 * 1000 * 1000
		v = strings.TrimSuffix(v, "TB")
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
