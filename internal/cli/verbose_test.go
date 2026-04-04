package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestVerboseOutputToStderr verifies that -v writes request/response details
// to stderr and not to stdout.
func TestVerboseOutputToStderr(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"msg":"hi"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, errOut := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/hello", "-v"}); err != nil {
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
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, _, errOut := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/hello"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("expected empty stderr without -v, got: %q", errOut.String())
	}
}
