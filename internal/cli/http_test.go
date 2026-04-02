package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// requestRecorder is a small helper that captures the last HTTP request
// a test server received, safe for concurrent access.
type requestRecorder struct {
	mu   sync.Mutex
	last *http.Request
}

func (rr *requestRecorder) capture(r *http.Request) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	// Copy the fields we need; the original request is owned by the server.
	rr.last = &http.Request{
		Method: r.Method,
		URL:    r.URL,
		Header: r.Header.Clone(),
	}
}

func (rr *requestRecorder) Last() *http.Request {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.last
}

// newTestServer starts an httptest.Server. The handler records each request
// via rr and responds with the given status and body.
func newTestServer(t *testing.T, rr *requestRecorder, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Consume body before clone to avoid a potential race with the
		// http server framework draining it after the handler returns.
		_ = r.Body
		rr.capture(r)
		w.WriteHeader(status)
		if body != "" {
			fmt.Fprint(w, body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestHTTPVerbs verifies that each lowercase verb sends the correct HTTP method.
func TestHTTPVerbs(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var rr requestRecorder
			srv := newTestServer(t, &rr, 200, "")
			c, _, _ := newTestCLI()
			if err := c.Run([]string{"restish", strings.ToLower(method), srv.URL}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := rr.Last().Method; got != method {
				t.Errorf("expected method %q, got %q", method, got)
			}
		})
	}
}

// TestHTTPVerbUppercaseAlias verifies that the uppercase alias (e.g. GET) also works.
func TestHTTPVerbUppercaseAlias(t *testing.T) {
	var rr requestRecorder
	srv := newTestServer(t, &rr, 200, "")
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "GET", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Method; got != "GET" {
		t.Errorf("expected GET, got %q", got)
	}
}

// TestBareURL verifies that a URL without an explicit verb is treated as GET.
func TestBareURL(t *testing.T) {
	var rr requestRecorder
	srv := newTestServer(t, &rr, 200, "")
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Method; got != "GET" {
		t.Errorf("expected GET for bare URL, got %q", got)
	}
}

// TestHTTPHeader verifies that -H adds the header to the request.
func TestHTTPHeader(t *testing.T) {
	var rr requestRecorder
	srv := newTestServer(t, &rr, 200, "")
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", "-H", "X-Test: hello", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("X-Test"); got != "hello" {
		t.Errorf("expected X-Test=hello, got %q", got)
	}
}

// TestHTTPHeaderRepeatable verifies that multiple -H flags all take effect.
func TestHTTPHeaderRepeatable(t *testing.T) {
	var rr requestRecorder
	srv := newTestServer(t, &rr, 200, "")
	c, _, _ := newTestCLI()
	err := c.Run([]string{"restish", "get",
		"-H", "X-First: one",
		"-H", "X-Second: two",
		srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := rr.Last()
	if got := req.Header.Get("X-First"); got != "one" {
		t.Errorf("expected X-First=one, got %q", got)
	}
	if got := req.Header.Get("X-Second"); got != "two" {
		t.Errorf("expected X-Second=two, got %q", got)
	}
}

// TestHTTPQuery verifies that -q appends a query parameter to the request.
func TestHTTPQuery(t *testing.T) {
	var rr requestRecorder
	srv := newTestServer(t, &rr, 200, "")
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", "-q", "foo=bar", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.Query().Get("foo"); got != "bar" {
		t.Errorf("expected query foo=bar, got %q", got)
	}
}

// TestHTTPServerOverride verifies that -s replaces the scheme and host.
func TestHTTPServerOverride(t *testing.T) {
	var rr requestRecorder
	srv := newTestServer(t, &rr, 200, "")
	c, _, _ := newTestCLI()
	// The URL argument points nowhere meaningful; -s redirects to our test server.
	err := c.Run([]string{"restish", "get", "-s", srv.URL, "https://api.example.com/items"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.Path; got != "/items" {
		t.Errorf("expected path /items after server override, got %q", got)
	}
}

// TestHTTPResponseBody verifies that the response body is written to stdout.
func TestHTTPResponseBody(t *testing.T) {
	var rr requestRecorder
	srv := newTestServer(t, &rr, 200, `{"hello":"world"}`)
	c, out, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"hello"`) {
		t.Errorf("expected response body in stdout, got: %q", out.String())
	}
}

// TestHTTPTimeout verifies that --rsh-timeout causes the request to fail
// when the server is too slow.
func TestHTTPTimeout(t *testing.T) {
	// A server that hangs until its context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI()
	err := c.Run([]string{"restish", "get", "--rsh-timeout", "50ms", srv.URL})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "network:") {
		t.Errorf("expected 'network:' prefix in error, got: %v", err)
	}
}

// TestHTTPInsecure verifies that --rsh-insecure disables TLS verification.
// The test server uses TLS; without --rsh-insecure the request should fail,
// with it the request should succeed.
func TestHTTPInsecure(t *testing.T) {
	var rr requestRecorder
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr.capture(r)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	// Without --rsh-insecure: TLS verification fails.
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", srv.URL}); err == nil {
		t.Error("expected TLS error without --rsh-insecure, got nil")
	}

	// With --rsh-insecure: request succeeds.
	c2, _, _ := newTestCLI()
	if err := c2.Run([]string{"restish", "get", "--rsh-insecure", srv.URL}); err != nil {
		t.Errorf("unexpected error with --rsh-insecure: %v", err)
	}

}
