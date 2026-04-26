package cli_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDiagnosticLabelsPlainWhenColorDisabled(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "--rsh-headers", "-f", "body", "https://api.example.com/hello"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	stderr := errOut.String()
	if !strings.Contains(stderr, `warning: --rsh-headers overrides -f; using "headers"`) {
		t.Fatalf("expected warning, got:\n%s", stderr)
	}
	if strings.Contains(stderr, "\x1b[") {
		t.Fatalf("expected plain warning without ANSI escapes, got:\n%q", stderr)
	}
}

func TestDiagnosticLabelsColorizedWhenColorEnabled(t *testing.T) {
	t.Setenv("COLOR", "1")
	t.Setenv("NO_COLOR", "")
	t.Setenv("NOCOLOR", "")

	c, _, errOut := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "--rsh-headers", "-f", "body", "https://api.example.com/hello"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	stderr := errOut.String()
	if !strings.Contains(stderr, "warning:") {
		t.Fatalf("expected warning label, got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "\x1b[") {
		t.Fatalf("expected colorized warning label, got:\n%q", stderr)
	}
}
