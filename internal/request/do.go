package request

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/cache"
	"github.com/gregjones/httpcache"
)

// Options controls per-request behavior derived from CLI flags.
type Options struct {
	// Headers is a list of "Name: Value" header strings to add to the request.
	Headers []string
	// Query is a list of "key=value" query parameter strings to append.
	Query []string
	// Server overrides the scheme and host (e.g. "https://staging.example.com").
	Server string
	// Insecure disables TLS certificate verification.
	Insecure bool
	// Timeout is the request timeout. Zero means no timeout.
	Timeout time.Duration
	// AcceptHeader, if non-empty, is sent as the Accept request header.
	AcceptHeader string
	// AcceptEncodingHeader, if non-empty, is sent as the Accept-Encoding header.
	AcceptEncodingHeader string
	// ContentType overrides the Content-Type header when a body is present.
	// If empty and a body is present, the caller is responsible for setting
	// the header via Headers.
	ContentType string
	// OnRequest, if non-nil, is called after all standard headers and query
	// params have been applied, immediately before the request is sent.
	// Auth handlers use this hook to inject credentials.
	OnRequest func(*http.Request) error
	// CacheDir, if non-empty, enables RFC 7234 response caching in that
	// directory.  NoCache overrides this and skips the cache entirely.
	CacheDir string
	// NoCache, when true, bypasses the response cache for this request
	// (no read, no write).
	NoCache bool
	// Retry is the maximum number of retry attempts for network errors and
	// 5xx responses.  Zero disables retries.
	Retry int
	// RetryBaseDelay is the base delay for the first retry backoff interval.
	// Defaults to 1 s when zero.
	RetryBaseDelay time.Duration
}

// Do executes an HTTP request and returns the response.
// The caller is responsible for closing resp.Body.
func Do(ctx context.Context, method, rawURL string, body io.Reader, opts Options) (*http.Response, error) {
	u, err := Normalize(rawURL, opts.Server)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	if opts.AcceptHeader != "" {
		req.Header.Set("Accept", opts.AcceptHeader)
	}
	if opts.AcceptEncodingHeader != "" {
		req.Header.Set("Accept-Encoding", opts.AcceptEncodingHeader)
	}

	// Apply extra headers. First colon separates name from value so header
	// values that contain colons are handled correctly.
	for _, h := range opts.Headers {
		name, value, ok := strings.Cut(h, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q: expected \"Name: Value\" format", h)
		}
		req.Header.Add(strings.TrimSpace(name), strings.TrimSpace(value))
	}

	// Append extra query parameters.
	if len(opts.Query) > 0 {
		q := req.URL.Query()
		for _, kv := range opts.Query {
			key, value, ok := strings.Cut(kv, "=")
			if !ok {
				return nil, fmt.Errorf("invalid query param %q: expected \"key=value\" format", kv)
			}
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	if opts.OnRequest != nil {
		if err := opts.OnRequest(req); err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
	}

	transport := buildTransport(opts)
	client := &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
	}

	return client.Do(req)
}

// newTransport returns an HTTP transport based on http.DefaultTransport.
// Cloning from the default preserves proxy settings (HTTP_PROXY, HTTPS_PROXY,
// NO_PROXY) and other production-appropriate defaults.
func newTransport(insecure bool) http.RoundTripper {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return tr
}

// buildTransport returns the appropriate RoundTripper for opts.
// Layer order (outermost → innermost):
//
//	httpcache.Transport → retryTransport → http.Transport
//
// The retry transport sits below the cache so that only cache misses (real
// server requests) are retried.
func buildTransport(opts Options) http.RoundTripper {
	base := newTransport(opts.Insecure)

	// Wrap with retry if requested.
	var inner http.RoundTripper = base
	if opts.Retry > 0 {
		delay := opts.RetryBaseDelay
		if delay == 0 {
			delay = time.Second
		}
		inner = retryTransport{inner: base, maxRetry: opts.Retry, baseDelay: delay}
	}

	if opts.NoCache || opts.CacheDir == "" {
		return inner
	}
	dc, err := cache.New(opts.CacheDir, cache.DefaultMaxBytes)
	if err != nil {
		// Cache unavailable; fall back without caching.
		return inner
	}
	ct := httpcache.NewTransport(dc)
	ct.Transport = inner
	return ct
}
