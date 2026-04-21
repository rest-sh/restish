package request

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/cache"
	"github.com/gregjones/httpcache"
)

type closeableTransport struct {
	inner    http.RoundTripper
	closeFns []func() error
}

func (t *closeableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.inner.RoundTrip(req)
}

func (t *closeableTransport) Close() error {
	var firstErr error
	for i := len(t.closeFns) - 1; i >= 0; i-- {
		if err := t.closeFns[i](); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

type closeAfterBody struct {
	io.ReadCloser
	closeFn func() error
}

func (c *closeAfterBody) Close() error {
	err := c.ReadCloser.Close()
	if closeErr := c.closeFn(); err == nil {
		err = closeErr
	}
	return err
}

// Options controls per-request behavior derived from CLI flags.
type Options struct {
	// BaseTransport, when non-nil, is the underlying transport to wrap with
	// TLS/cache/retry behavior. This is primarily useful in tests that want to
	// avoid real network listeners.
	BaseTransport http.RoundTripper
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
	// ClientCertPath is the PEM client certificate path for mTLS.
	ClientCertPath string
	// ClientKeyPath is the PEM client private key path for mTLS.
	ClientKeyPath string
	// TLSSignerPath is the executable path of a tls-signer plugin for mTLS.
	TLSSignerPath string
	// TLSSignerName records the logical signer name before CLI resolution.
	TLSSignerName string
	// TLSSignerParams holds plugin-specific configuration for the tls-signer.
	TLSSignerParams map[string]string
	// CACertPath is an optional PEM CA bundle to trust in addition to system roots.
	CACertPath string
	// TLSMinVersion constrains the minimum TLS version when connecting over HTTPS.
	TLSMinVersion uint16
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
	// OnUnauthorized, when non-nil, is used by callers that want to retry once
	// after a 401 with freshly acquired credentials.
	OnUnauthorized func(*http.Request) error
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
	// Transport, if non-nil, is reused for every request instead of building
	// a new one from the TLS/cache/retry options. Callers that make multiple
	// requests with the same options (e.g. pagination) should pre-build a
	// transport via BuildTransport and set it here so that the underlying
	// http.Transport's connection pool is reused across requests.
	Transport http.RoundTripper
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

	transport := opts.Transport
	builtTransport := false
	if transport == nil {
		transport = BuildTransport(opts)
		builtTransport = true
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		if builtTransport {
			if closer, ok := transport.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		}
		return nil, err
	}
	if builtTransport {
		if closer, ok := transport.(interface{ Close() error }); ok && resp.Body != nil {
			resp.Body = &closeAfterBody{ReadCloser: resp.Body, closeFn: closer.Close}
		}
	}
	return resp, nil
}

// newTransport returns an HTTP transport based on http.DefaultTransport.
// Cloning from the default preserves proxy settings (HTTP_PROXY, HTTPS_PROXY,
// NO_PROXY) and other production-appropriate defaults.
func newTransport(opts Options) (http.RoundTripper, error) {
	if opts.BaseTransport != nil {
		if tr, ok := opts.BaseTransport.(*http.Transport); ok {
			cloned := tr.Clone()
			cfg, cleanup, err := TLSConfigWithCleanupFromOptions(opts)
			if err != nil {
				return nil, err
			}
			if cfg.InsecureSkipVerify || cfg.MinVersion != 0 || len(cfg.Certificates) > 0 || cfg.RootCAs != nil {
				cloned.TLSClientConfig = cfg
			}
			return wrapTransportWithCleanup(cloned, cleanup), nil
		}
		if hasTLSOptions(opts) {
			return nil, fmt.Errorf("custom base transport does not support TLS option overrides")
		}
		return opts.BaseTransport, nil
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	cfg, cleanup, err := TLSConfigWithCleanupFromOptions(opts)
	if err != nil {
		return nil, err
	}
	if cfg.InsecureSkipVerify || cfg.MinVersion != 0 || len(cfg.Certificates) > 0 || cfg.RootCAs != nil {
		tr.TLSClientConfig = cfg
	}
	return wrapTransportWithCleanup(tr, cleanup), nil
}

func hasTLSOptions(opts Options) bool {
	return opts.Insecure ||
		opts.ClientCertPath != "" ||
		opts.ClientKeyPath != "" ||
		opts.TLSSignerPath != "" ||
		opts.CACertPath != "" ||
		opts.TLSMinVersion != 0
}

// BuildTransport returns the appropriate RoundTripper for opts.
// Layer order (outermost → innermost):
//
//	httpcache.Transport → retryTransport → http.Transport
//
// The retry transport sits below the cache so that only cache misses (real
// server requests) are retried.
func BuildTransport(opts Options) http.RoundTripper {
	base, err := newTransport(opts)
	if err != nil {
		// TLS config invalid; use a small transport that returns the config error.
		return roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return nil, err
		})
	}

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
	return wrapTransportWithCloseFns(ct, transportCleanup(inner)...)
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func wrapTransportWithCleanup(rt http.RoundTripper, cleanup io.Closer) http.RoundTripper {
	closeFns := transportCleanup(rt)
	if cleanup != nil {
		closeFns = append(closeFns, cleanup.Close)
	}
	return wrapTransportWithCloseFns(rt, closeFns...)
}

func wrapTransportWithCloseFns(rt http.RoundTripper, closeFns ...func() error) http.RoundTripper {
	if len(closeFns) == 0 {
		return rt
	}
	return &closeableTransport{inner: rt, closeFns: closeFns}
}

func transportCleanup(rt http.RoundTripper) []func() error {
	var closeFns []func() error
	if closer, ok := rt.(interface{ Close() error }); ok {
		closeFns = append(closeFns, closer.Close)
	}
	if idleCloser, ok := rt.(interface{ CloseIdleConnections() }); ok {
		closeFns = append(closeFns, func() error {
			idleCloser.CloseIdleConnections()
			return nil
		})
	}
	return closeFns
}
