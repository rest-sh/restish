package cli_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/cli"
)

// jsonServer starts an httptest.Server that responds with status and a JSON body.
func jsonServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestJSONOutput verifies that a non-TTY invocation outputs the body as JSON.
func TestJSONOutput(t *testing.T) {
	srv := jsonServer(t, 200, `{"name":"Alice","score":42}`)
	c, out, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must be valid JSON.
	var v any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}

	// Must be the body only (no "status" key at the top level).
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", v)
	}
	if m["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", m["name"])
	}
	if _, hasStatus := m["status"]; hasStatus {
		t.Error("json output should contain body only, not full response struct")
	}
}

// TestReadableOutput verifies that -o readable includes the status line and headers.
func TestReadableOutput(t *testing.T) {
	srv := jsonServer(t, 200, `{"hello":"world"}`)
	c, out, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", "-o", "readable", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()

	if !strings.Contains(got, "200") {
		t.Errorf("readable output missing status code:\n%s", got)
	}
	if !strings.Contains(got, "Content-Type") {
		t.Errorf("readable output missing Content-Type header:\n%s", got)
	}
}

// TestReadableBodyIsValidJSON verifies that the body section of -o readable output
// is parseable JSON (no ANSI codes since we're not a TTY).
func TestReadableBodyIsValidJSON(t *testing.T) {
	srv := jsonServer(t, 200, `{"key":"value"}`)
	c, out, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", "-o", "readable", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Split on the blank line between headers and body.
	parts := strings.SplitN(out.String(), "\n\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected blank-line separator in readable output:\n%s", out.String())
	}
	bodyPart := strings.TrimSpace(parts[1])

	var v any
	if err := json.Unmarshal([]byte(bodyPart), &v); err != nil {
		t.Errorf("body section is not valid JSON: %v\nbody: %s", err, bodyPart)
	}
}

// TestExitCode4xx verifies that a 4xx response returns ExitCodeError{Code:4}.
func TestExitCode4xx(t *testing.T) {
	srv := jsonServer(t, 404, `{"error":"not found"}`)
	c, _, _ := newTestCLI()
	err := c.Run([]string{"restish", "get", srv.URL})

	var exitErr *cli.ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 4 {
		t.Errorf("expected exit code 4, got %d", exitErr.Code)
	}
}

// TestExitCode5xx verifies that a 5xx response returns ExitCodeError{Code:5}.
func TestExitCode5xx(t *testing.T) {
	srv := jsonServer(t, 500, `{"error":"boom"}`)
	c, _, _ := newTestCLI()
	err := c.Run([]string{"restish", "get", srv.URL})

	var exitErr *cli.ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 5 {
		t.Errorf("expected exit code 5, got %d", exitErr.Code)
	}
}

// TestExitCode2xx verifies that a 2xx response returns nil.
func TestExitCode2xx(t *testing.T) {
	srv := jsonServer(t, 200, `{}`)
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Errorf("expected nil error for 200, got: %v", err)
	}
}

// TestIgnoreStatusCode verifies that --rsh-ignore-status-code returns nil
// even for 4xx/5xx responses.
func TestIgnoreStatusCode(t *testing.T) {
	srv := jsonServer(t, 500, `{"error":"server error"}`)
	c, _, _ := newTestCLI()
	err := c.Run([]string{"restish", "get", "--rsh-ignore-status-code", srv.URL})
	if err != nil {
		t.Errorf("expected nil with --rsh-ignore-status-code, got: %v", err)
	}
}

// TestUnknownOutputFormat verifies that -o nosuchformat returns an error.
func TestUnknownOutputFormat(t *testing.T) {
	srv := jsonServer(t, 200, `{}`)
	c, _, _ := newTestCLI()
	err := c.Run([]string{"restish", "get", "-o", "nosuchformat", srv.URL})
	if err == nil {
		t.Fatal("expected error for unknown output format, got nil")
	}
}

// TestResponseBodyOnError verifies that a 4xx response still writes the body
// to stdout before returning the exit code error.
func TestResponseBodyOnError(t *testing.T) {
	srv := jsonServer(t, 404, `{"error":"not found"}`)
	c, out, _ := newTestCLI()
	_ = c.Run([]string{"restish", "get", srv.URL}) // ignore the ExitCodeError

	var v any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Errorf("expected body output even on 404, got invalid output: %q", out.String())
	}
}
