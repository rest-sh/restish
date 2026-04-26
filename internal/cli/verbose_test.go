package cli_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestVerboseOutputToStderr verifies that -v writes request/response details
// to stderr and not to stdout.
func TestVerboseOutputToStderr(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"msg":"hi"}`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "-v", "https://api.example.com/hello"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	stderr := errOut.String()
	stdout := out.String()

	// stderr must contain request line.
	if !strings.Contains(stderr, "> GET") {
		t.Errorf("expected '> GET' in stderr, got:\n%s", stderr)
	}
	// stderr must contain response status.
	if !strings.Contains(stderr, "< HTTP") || !strings.Contains(stderr, "200") {
		t.Errorf("expected '< HTTP/1.1 200' in stderr, got:\n%s", stderr)
	}
	// Verbose lines must NOT appear in stdout.
	if strings.Contains(stdout, "> GET") || strings.Contains(stdout, "< HTTP") {
		t.Errorf("verbose output leaked to stdout:\n%s", stdout)
	}
	// The response body should still appear in stdout (or be formatted normally).
	if !strings.Contains(stdout, "hi") {
		t.Errorf("expected response body in stdout, got:\n%s", stdout)
	}
}

// TestNonVerboseOutputClean verifies that without -v, stderr is empty for
// a successful request.
func TestNonVerboseOutputClean(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/hello"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("expected empty stderr without -v, got: %q", errOut.String())
	}
}

func TestVerboseRedactsSensitiveQueryParams(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "-v", "https://api.example.com/hello?api_key=secret&token=abc&page=1"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	stderr := errOut.String()
	if strings.Contains(stderr, "api_key=secret") || strings.Contains(stderr, "token=abc") {
		t.Fatalf("expected sensitive query values redacted, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "api_key=%3Credacted%3E") || !strings.Contains(stderr, "token=%3Credacted%3E") {
		t.Fatalf("expected redacted query params in verbose output, got:\n%s", stderr)
	}
}

func TestVerboseRedactsSensitiveResponseHeaders(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Set-Cookie":   []string{"session=secret"},
				"X-Request-Id": []string{"visible"},
			},
			Body:    io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request: r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "-v", "https://api.example.com/hello"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	stderr := errOut.String()
	if strings.Contains(stderr, "session=secret") {
		t.Fatalf("verbose response leaked Set-Cookie:\n%s", stderr)
	}
	if !strings.Contains(stderr, "< Set-Cookie: <redacted>") {
		t.Fatalf("expected redacted Set-Cookie, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "< X-Request-Id: visible") {
		t.Fatalf("expected non-sensitive header to remain visible, got:\n%s", stderr)
	}
}

func TestVerboseRedactsJSONBodies(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"token":"response-secret","ok":true}`)),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "post", "-v", "https://api.example.com/hello", "token: request-secret"})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	stderr := errOut.String()
	if strings.Contains(stderr, "request-secret") || strings.Contains(stderr, "response-secret") {
		t.Fatalf("verbose body leaked secret:\n%s", stderr)
	}
	if strings.Count(stderr, `\u003credacted\u003e`) < 2 && strings.Count(stderr, "<redacted>") < 2 {
		t.Fatalf("expected redacted request and response bodies, got:\n%s", stderr)
	}
}

func TestVerboseLogsRequestBodyBeforeResponseHeaders(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 201,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "post", "-v", "https://api.example.com/hello", "name: Ada"}); err != nil {
		t.Fatalf("post: %v", err)
	}
	stderr := errOut.String()
	bodyAt := strings.Index(stderr, "> body:")
	responseAt := strings.Index(stderr, "< HTTP/1.1 201")
	if bodyAt < 0 || responseAt < 0 {
		t.Fatalf("expected request body and response headers in verbose output, got:\n%s", stderr)
	}
	if bodyAt > responseAt {
		t.Fatalf("expected request body before response headers, got:\n%s", stderr)
	}
}

func TestVerboseLogsCachedResponses(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	c1, _, errOut1 := newTestCLI(t)
	c1.Hooks().CachePath = cacheDir
	if err := c1.Run([]string{"restish", "get", "-v", srv.URL}); err != nil {
		t.Fatalf("first get: %v", err)
	}
	if strings.Contains(errOut1.String(), "* Cache: HIT") {
		t.Fatalf("first request should not be marked as cache hit:\n%s", errOut1.String())
	}

	c2, _, errOut2 := newTestCLI(t)
	c2.Hooks().CachePath = cacheDir
	if err := c2.Run([]string{"restish", "get", "-v", srv.URL}); err != nil {
		t.Fatalf("second get: %v", err)
	}
	stderr := errOut2.String()
	for _, want := range []string{"> GET ", "< HTTP/1.1 200 OK", "* Cache: HIT"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("cached verbose output missing %q:\n%s", want, stderr)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server hits = %d, want 1", got)
	}
}

func TestVerboseLogsEveryPaginatedRequest(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useThreePageTransport(c)

	if err := c.Run([]string{"restish", "get", "-v", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	stderr := errOut.String()
	if got := strings.Count(stderr, "> GET "); got != 3 {
		t.Fatalf("expected verbose output for 3 requests, got %d:\n%s", got, stderr)
	}
	if got := strings.Count(stderr, "< HTTP/1.1 200 OK"); got != 3 {
		t.Fatalf("expected verbose output for 3 responses, got %d:\n%s", got, stderr)
	}
	for _, want := range []string{
		"https://api.example.com/items",
		"https://api.example.com/items?page=2",
		"https://api.example.com/items?page=3",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected verbose output to include %s, got:\n%s", want, stderr)
		}
	}
}

// TestVerboseTLSDetailsAtLevel2 verifies that -vv (verbose >= 2) prints TLS
// version, cipher suite, and peer certificate information to stderr.
func TestVerboseTLSDetailsAtLevel2(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    r,
			TLS: &tls.ConnectionState{
				Version:     tls.VersionTLS13,
				CipherSuite: tls.TLS_AES_256_GCM_SHA384,
			},
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "-vv", "https://api.example.com"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	stderr := errOut.String()
	if !strings.Contains(stderr, "TLS 1.3") {
		t.Errorf("expected TLS version in -vv stderr, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "TLS_AES_256_GCM_SHA384") {
		t.Errorf("expected cipher suite in -vv stderr, got:\n%s", stderr)
	}
}

// TestVerboseTLSDetailsNotAtLevel1 verifies that -v (verbose == 1) does NOT
// print TLS details, keeping the output concise.
func TestVerboseTLSDetailsNotAtLevel1(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    r,
			TLS: &tls.ConnectionState{
				Version:     tls.VersionTLS13,
				CipherSuite: tls.TLS_AES_256_GCM_SHA384,
			},
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "-v", "https://api.example.com"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	stderr := errOut.String()
	if strings.Contains(stderr, "TLS 1.3") {
		t.Errorf("unexpected TLS details at -v level:\n%s", stderr)
	}
}
