package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rest-sh/restish/v2/internal/hypermedia"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
)

type preparedRequest struct {
	rawURL  string
	apiName string
	opts    request.Options
	body    io.Reader
	bodyRaw []byte
}

func (c *CLI) prepareRequest(
	rawURL, profileName string,
	opts request.Options,
	bodyValue any,
	extraHeaders []string,
	noAuth bool,
	authOpts authHandlerOptions,
	operationAuth *operationAuthPolicy,
) (*preparedRequest, error) {
	opts = cloneRequestOptions(opts)
	if len(extraHeaders) > 0 {
		opts.Headers = append(opts.Headers, extraHeaders...)
	}

	rawURL, apiName, opts, err := c.applyAPIProfile(rawURL, profileName, opts, authOpts)
	if err != nil {
		return nil, err
	}
	if noAuth && operationAuth != nil && strings.TrimSpace(operationAuth.Override) != "" {
		return nil, fmt.Errorf("auth override %q is not valid for an operation with security: []", operationAuth.Override)
	}
	if !noAuth && operationAuth != nil && apiName != "" {
		prof := c.cfg.APIs[apiName].Profiles[profileName]
		selected, handled, err := c.planOperationAuth(apiName, profileName, prof, operationAuth)
		if err != nil {
			return nil, err
		}
		if handled {
			callbacks, err := c.operationAuthCallbacks(apiName, profileName, selected, authOpts)
			if err != nil {
				return nil, err
			}
			opts.OnRequest = callbacks.OnRequest
			opts.OnUnauthorized = callbacks.OnUnauthorized
		}
	}
	opts, err = c.resolveTLSSigner(opts)
	if err != nil {
		return nil, err
	}

	// When following a cross-host redirect, strip credentials to prevent
	// a compromised plugin from issuing credentialed requests to arbitrary hosts.
	if noAuth {
		opts.OnRequest = nil
		opts.OnUnauthorized = nil
		filtered := opts.Headers[:0]
		for _, h := range opts.Headers {
			name, _, _ := strings.Cut(h, ":")
			if !isSensitiveHeader(name) {
				filtered = append(filtered, h)
			}
		}
		opts.Headers = filtered
		opts.Query = filterCredentialQueryParams(opts.Query)
	}

	if opts.OnRequest != nil || requestOptionHeadersContainCredentials(opts.Headers) {
		opts.NoCache = true
	}

	// Build the transport once so follow-up requests can reuse the same
	// connection pool via the returned opts value.
	opts.Transport = request.BuildTransport(opts)
	if closer, ok := opts.Transport.(io.Closer); ok {
		c.registerRequestCloser(closer)
	}

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

	bodyRaw, err := c.requestBodyBytes(opts.ContentType, bodyValue, &opts.Headers)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	var body io.Reader
	if len(bodyRaw) > 0 {
		body = bytes.NewReader(bodyRaw)
	}

	return &preparedRequest{
		rawURL:  rawURL,
		apiName: apiName,
		opts:    opts,
		body:    body,
		bodyRaw: bodyRaw,
	}, nil
}

func filterCredentialQueryParams(query []string) []string {
	if len(query) == 0 {
		return query
	}
	filtered := query[:0]
	for _, kv := range query {
		name, _, ok := strings.Cut(kv, "=")
		if !ok {
			filtered = append(filtered, kv)
			continue
		}
		decoded, err := url.QueryUnescape(name)
		if err == nil {
			name = decoded
		}
		if !isSensitiveQueryParam(name) {
			filtered = append(filtered, kv)
		}
	}
	return filtered
}

func requestOptionHeadersContainCredentials(headers []string) bool {
	for _, h := range headers {
		name, _, _ := strings.Cut(h, ":")
		if isSensitiveHeader(name) {
			return true
		}
	}
	return false
}

func cloneRequestOptions(opts request.Options) request.Options {
	cloned := opts
	if len(opts.Headers) > 0 {
		cloned.Headers = append([]string(nil), opts.Headers...)
	}
	if len(opts.Query) > 0 {
		cloned.Query = append([]string(nil), opts.Query...)
	}
	if len(opts.TLSSignerParams) > 0 {
		cloned.TLSSignerParams = make(map[string]string, len(opts.TLSSignerParams))
		for k, v := range opts.TLSSignerParams {
			cloned.TLSSignerParams[k] = v
		}
	}
	return cloned
}

func (c *CLI) requestBodyBytes(contentType string, bodyValue any, headers *[]string) ([]byte, error) {
	if bodyValue == nil {
		return nil, nil
	}

	ct := contentType
	if ct == "" {
		ct = "application/json"
	}
	mimeType := c.content.MIMETypeForName(ct)
	if mimeType == "" {
		mimeType = ct
	}
	encoded, actualContentType, err := c.content.EncodeWithType(mimeType, bodyValue)
	if err != nil {
		return nil, err
	}
	*headers = append(*headers, "Content-Type: "+actualContentType)
	return encoded, nil
}

func (c *CLI) sendPreparedRequest(ctx context.Context, method string, prepared *preparedRequest) (*http.Response, error) {
	bodyReader := func() io.Reader {
		if len(prepared.bodyRaw) == 0 {
			return nil
		}
		return bytes.NewReader(prepared.bodyRaw)
	}
	resp, err := request.Do(ctx, method, prepared.rawURL, bodyReader(), prepared.opts)
	if err != nil || resp == nil || resp.StatusCode != http.StatusUnauthorized || prepared.opts.OnUnauthorized == nil {
		return resp, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	retryOpts := cloneRequestOptions(prepared.opts)
	retryOpts.Transport = prepared.opts.Transport
	onUnauthorized := retryOpts.OnUnauthorized
	retryOpts.OnRequest = func(req *http.Request) error {
		if err := onUnauthorized(req); err != nil {
			return err
		}
		return c.runRequestMiddlewarePlugins(req)
	}
	return request.Do(ctx, method, prepared.rawURL, bodyReader(), retryOpts)
}

func (c *CLI) closePreparedTransport(prepared *preparedRequest) {
	if prepared == nil || prepared.opts.Transport == nil {
		return
	}
	if closer, ok := prepared.opts.Transport.(io.Closer); ok {
		_ = closer.Close()
	}
}

func (c *CLI) normalizeHTTPResponse(httpResp *http.Response, maxBodyBytes int64) (*output.Response, error) {
	resp, err := output.Normalize(httpResp, c.content, maxBodyBytes)
	if err != nil {
		return nil, err
	}

	// httpResp headers/request are still accessible after Normalize has closed
	// and consumed the body.
	if httpResp.Request != nil {
		if links := hypermedia.Parse(httpResp.Request.URL, httpResp.Header, nil, []hypermedia.Parser{hypermedia.LinkHeaderParser{}}); len(links) > 0 {
			resp.Links = make(map[string]any, len(links))
			for k, v := range links {
				resp.Links[k] = v
			}
		}
	}

	return resp, nil
}

func (c *CLI) ensureBodyLinks(resp *output.Response) {
	if resp == nil || resp.URL == "" {
		return
	}
	base, err := url.Parse(resp.URL)
	if err != nil {
		return
	}
	headers := make(http.Header, len(resp.Headers))
	for k, v := range resp.Headers {
		headers.Set(k, v)
	}
	links := hypermedia.Parse(base, headers, resp.Body, c.linkParsers)
	if len(links) == 0 {
		return
	}
	if resp.Links == nil {
		resp.Links = make(map[string]any, len(links))
	}
	for k, v := range links {
		resp.Links[k] = v
	}
}
