package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
)

func useJSONResponse(c *cli.CLI, status int, body string) {
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestJSONOutput verifies that a non-TTY invocation outputs the body as JSON.
func TestJSONOutput(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"name":"Alice","score":42}`)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
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
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"hello":"world"}`)
	if err := c.Run([]string{"restish", "get", "-o", "readable", "https://api.example.com/items"}); err != nil {
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
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"key":"value"}`)
	if err := c.Run([]string{"restish", "get", "-o", "readable", "https://api.example.com/items"}); err != nil {
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
	c, _, _ := newTestCLI(t)
	useJSONResponse(c, 404, `{"error":"not found"}`)
	err := c.Run([]string{"restish", "get", "https://api.example.com/items"})

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
	c, _, _ := newTestCLI(t)
	useJSONResponse(c, 500, `{"error":"boom"}`)
	err := c.Run([]string{"restish", "get", "https://api.example.com/items"})

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
	c, _, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{}`)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Errorf("expected nil error for 200, got: %v", err)
	}
}

// TestIgnoreStatusCode verifies that --rsh-ignore-status-code returns nil
// even for 4xx/5xx responses.
func TestIgnoreStatusCode(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useJSONResponse(c, 500, `{"error":"server error"}`)
	err := c.Run([]string{"restish", "get", "--rsh-ignore-status-code", "https://api.example.com/items"})
	if err != nil {
		t.Errorf("expected nil with --rsh-ignore-status-code, got: %v", err)
	}
}

// TestUnknownOutputFormat verifies that -o nosuchformat returns an error.
func TestUnknownOutputFormat(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{}`)
	err := c.Run([]string{"restish", "get", "-o", "nosuchformat", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected error for unknown output format, got nil")
	}
}

// TestResponseBodyOnError verifies that a 4xx response still writes the body
// to stdout before returning the exit code error.
func TestResponseBodyOnError(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 404, `{"error":"not found"}`)
	_ = c.Run([]string{"restish", "get", "https://api.example.com/items"}) // ignore the ExitCodeError

	var v any
	if err := json.Unmarshal(out.Bytes(), &v); err != nil {
		t.Errorf("expected body output even on 404, got invalid output: %q", out.String())
	}
}

// TestFilterShorthand verifies that -f body.name extracts the right field.
func TestFilterShorthand(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"name":"Alice","age":30}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.name", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	// JSON-encoded string result.
	if got != `"Alice"` {
		t.Errorf("got %q, want %q", got, `"Alice"`)
	}
}

// TestFilterRaw verifies that -r strips quotes from string results.
func TestFilterRaw(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"name":"Alice"}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.name", "-r", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got != "Alice" {
		t.Errorf("got %q, want %q", got, "Alice")
	}
}

// TestFilterHeaders verifies that --rsh-headers is shorthand for -f headers.
func TestFilterHeaders(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{}`)
	if err := c.Run([]string{"restish", "get", "--rsh-headers", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Content-Type") {
		t.Errorf("expected headers in output, got: %s", out.String())
	}
}

func TestFilterHeaderValue(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Date":         []string{"Mon, 02 Jan 2006 15:04:05 GMT"},
			},
			Body:    io.NopCloser(strings.NewReader(`{}`)),
			Request: r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "-f", "headers.Date", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != `"Mon, 02 Jan 2006 15:04:05 GMT"` {
		t.Fatalf("headers.Date output = %q", got)
	}
}

func TestFilterTopLevelRootsDoNotTriggerBodyHint(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	useJSONResponse(c, 200, `{}`)
	if err := c.Run([]string{"restish", "get", "-f", "links", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(errOut.String(), "filter returned no results") {
		t.Fatalf("expected no body hint for top-level links filter, got: %q", errOut.String())
	}
}

func TestFilterArraySyntaxDoesNotTriggerBodyHint(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	useJSONResponse(c, 200, `{"items":[{"name":"Alice"}]}`)
	if err := c.Run([]string{"restish", "get", "-f", "body[0]", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(errOut.String(), "filter returned no results") {
		t.Fatalf("expected no body hint for body[0], got: %q", errOut.String())
	}
}

func TestHeadersFlagWarnsWhenOverridingFilter(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	useJSONResponse(c, 200, `{}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.name", "--rsh-headers", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(errOut.String(), "--rsh-headers overrides -f") {
		t.Fatalf("expected override warning, got: %q", errOut.String())
	}
}

func TestFilterAtNonTTYUsesJSONFormatter(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"name":"Alice"}`)
	if err := c.Run([]string{"restish", "get", "-f", "@", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("expected JSON output for -f @ on non-TTY, got %q: %v", out.String(), err)
	}
	if decoded["status"] != float64(200) {
		t.Fatalf("expected full response status, got %#v", decoded)
	}
	body, ok := decoded["body"].(map[string]any)
	if !ok || body["name"] != "Alice" {
		t.Fatalf("unexpected response body: %#v", decoded)
	}
}

func TestRawFlagWithoutFilterWritesOriginalBytes(t *testing.T) {
	raw := "{\n  \"z\": 1,\n  \"a\": 2\n}\n"
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, raw)
	if err := c.Run([]string{"restish", "get", "-r", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.String() != raw {
		t.Fatalf("raw output changed:\ngot  %q\nwant %q", out.String(), raw)
	}
}

func TestImageResponseDefaultNonTTYWritesOriginalBytes(t *testing.T) {
	data := pngBytes(t)
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(data)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/logo.png"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("image bytes changed: got %d bytes, want %d", out.Len(), len(data))
	}
	if _, err := png.Decode(bytes.NewReader(out.Bytes())); err != nil {
		t.Fatalf("output is not a valid png: %v", err)
	}
}
