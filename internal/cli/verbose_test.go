package cli_test

import (
	"crypto/tls"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestVerboseOutputToStderr verifies that -v writes request/response details
// to stderr and not to stdout.
func TestVerboseOutputToStderr(t *testing.T) {
	c, out, errOut := newTestCLI()
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
	c, _, errOut := newTestCLI()
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
	c, _, errOut := newTestCLI()
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

// TestVerboseTLSDetailsAtLevel2 verifies that -vv (verbose >= 2) prints TLS
// version, cipher suite, and peer certificate information to stderr.
func TestVerboseTLSDetailsAtLevel2(t *testing.T) {
	c, _, errOut := newTestCLI()
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
	c, _, errOut := newTestCLI()
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
