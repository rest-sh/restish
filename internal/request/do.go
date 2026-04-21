package request

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/cache"
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
	// Headers is a list of "Name: Value" header strings to add to the request.
	Headers []string
	// Query is a list of "key=value" query parameter strings to append.
	Query []string
	// Server overrides the scheme and host (e.g. "https://staging.example.com").
	Server string
	// Insecure disables TLS certificate verification.
	Insecure bool
	// Timeout bounds the time to receive response headers. Zero means no timeout.
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
	// CacheNamespace partitions cache entries for one API/profile tuple.
	CacheNamespace string
	// CacheMaxBytes is the maximum size of the HTTP response cache in bytes.
	// If zero, defaults to cache.DefaultMaxBytes.
	CacheMaxBytes int64
	// Retry is the maximum number of retry attempts for network errors and
	// 5xx responses.  Zero disables retries.
	Retry int
	// RetryBaseDelay is the base delay for the first retry backoff interval.
	// Defaults to 1 s when zero.
	RetryBaseDelay time.Duration
	// Logger receives retry progress warnings on stderr-style output.
	Logger io.Writer
	// Transport, when passed to BuildTransport, is the underlying transport to
	// wrap with TLS/cache/retry behavior. When passed to Do, it is treated as a
	// fully built transport and reused as-is. Callers that make multiple
	// requests with the same options (e.g. pagination) should pre-build one via
	// BuildTransport and set it here so connection pools are reused.
	Transport http.RoundTripper
}

// Do executes an HTTP request and returns the response.
// The caller is responsible for closing resp.Body.
func Do(ctx context.Context, method, rawURL string, body io.Reader, opts Options) (*http.Response, error) {
	u, err := Normalize(rawURL, opts.Server)
	if err != nil {
		return nil, err
	}

	requestCtx := ctx
	var cancelRequest context.CancelCauseFunc
	if opts.Timeout > 0 {
		requestCtx, cancelRequest = context.WithCancelCause(ctx)
		defer func() {
			if err != nil && cancelRequest != nil {
				cancelRequest(context.Canceled)
			}
		}()
	}

	req, err := http.NewRequestWithContext(requestCtx, method, u, body)
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
	}

	resp, err := doWithHeaderTimeout(client, req, opts.Timeout, cancelRequest)
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

type doResult struct {
	resp *http.Response
	err  error
}

func doWithHeaderTimeout(client *http.Client, req *http.Request, timeout time.Duration, cancel context.CancelCauseFunc) (*http.Response, error) {
	if timeout <= 0 {
		return client.Do(req)
	}

	resultCh := make(chan doResult, 1)
	go func() {
		resp, err := client.Do(req)
		resultCh <- doResult{resp: resp, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result.resp, result.err
	case <-req.Context().Done():
		if cancel != nil {
			cancel(context.Cause(req.Context()))
		}
		result := <-resultCh
		if result.err == nil {
			if result.resp != nil && result.resp.Body != nil {
				_ = result.resp.Body.Close()
			}
			return nil, req.Context().Err()
		}
		return nil, result.err
	case <-timer.C:
		if cancel != nil {
			cancel(context.DeadlineExceeded)
		}
		result := <-resultCh
		if result.err == nil {
			if result.resp != nil && result.resp.Body != nil {
				_ = result.resp.Body.Close()
			}
		}
		return nil, context.DeadlineExceeded
	}
}

// newTransport returns an HTTP transport based on http.DefaultTransport.
// Cloning from the default preserves proxy settings (HTTP_PROXY, HTTPS_PROXY,
// NO_PROXY) and other production-appropriate defaults.
func newTransport(opts Options) (http.RoundTripper, error) {
	if opts.Transport != nil {
		if tr, ok := opts.Transport.(*http.Transport); ok {
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
		return opts.Transport, nil
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
		inner = retryTransport{inner: base, maxRetry: opts.Retry, baseDelay: delay, logger: opts.Logger}
	}

	if opts.NoCache || opts.CacheDir == "" {
		return inner
	}
	maxBytes := opts.CacheMaxBytes
	if maxBytes == 0 {
		maxBytes = cache.DefaultMaxBytes
	}
	dc, err := cache.New(opts.CacheDir, maxBytes, opts.CacheNamespace)
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
