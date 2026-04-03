package request

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
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

	client := &http.Client{
		Transport: newTransport(opts.Insecure),
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
