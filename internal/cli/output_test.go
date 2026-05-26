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

	"github.com/fxamacker/cbor/v2"
	"github.com/rest-sh/restish/v2/internal/cli"
)

func useCBORResponse(t *testing.T, c *cli.CLI, status int, value any) []byte {
	t.Helper()
	raw, err := cbor.Marshal(value)
	if err != nil {
		t.Fatalf("marshal cbor: %v", err)
	}
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/cbor"}},
			Body:       io.NopCloser(bytes.NewReader(raw)),
			Request:    r,
		}, nil
	})
	return raw
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

func TestDefaultTTYOutputPrintsResponseTranscript(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type":  []string{"application/json"},
				"Set-Cookie":    []string{"session=secret"},
				"X-Trace-Token": []string{"abc123"},
			},
			Body:    io.NopCloser(strings.NewReader(`{"hello":"world"}`)),
			Request: r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	context := stripANSI(out.String())
	requireContains(t, context, "HTTP/1.1 200 OK", "Content-Type: application/json", "X-Trace-Token: abc123")
	if strings.Contains(context, "session=secret") || !strings.Contains(context, "Set-Cookie: <redacted>") {
		t.Fatalf("stdout did not redact sensitive header:\n%s", context)
	}
	if !strings.Contains(context, `"hello": "world"`) {
		t.Fatalf("stdout missing formatted body:\n%s", context)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr diagnostics, got:\n%s", errOut.String())
	}
}

func TestExplicitJSONTTYOutputStillShowsContextOnStdout(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	useJSONResponse(c, 200, `{"key":"value"}`)
	if err := c.Run([]string{"restish", "get", "-o", "json", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := stripANSI(out.String())
	if !strings.Contains(got, "HTTP/1.1 200 OK") || !strings.Contains(got, "Content-Type: application/json") || !strings.Contains(got, `"key": "value"`) {
		t.Fatalf("stdout missing response transcript:\n%s", got)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr diagnostics, got:\n%s", errOut.String())
	}
}

func TestExplicitPrintBodyOnlySuppressesTTYHeaders(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	useJSONResponse(c, 200, `{"key":"value"}`)
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", "-o", "json", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "HTTP/1.1") || strings.Contains(got, "Content-Type:") {
		t.Fatalf("body-only print included headers:\n%s", got)
	}
	var body map[string]string
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatalf("stdout should be body JSON only, got %q: %v", out.String(), err)
	}
	if body["key"] != "value" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestExplicitPrintBodyColorColorsCompactJSON(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"key":"value"}`)
	if err := c.Run([]string{"restish", "get", "--rsh-print", "bc", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected colored compact JSON, got:\n%s", got)
	}
	stripped := stripANSI(got)
	var body map[string]string
	if err := json.Unmarshal([]byte(stripped), &body); err != nil {
		t.Fatalf("colored output should strip to JSON, got %q: %v", stripped, err)
	}
	if body["key"] != "value" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestExplicitPrintBodyPrettyNoColorOnTTY(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	useJSONResponse(c, 200, `{"key":"value"}`)
	if err := c.Run([]string{"restish", "get", "--rsh-print", "bp", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("expected no ANSI color codes for --rsh-print=bp, got:\n%s", got)
	}
	if !strings.Contains(got, "key") || !strings.Contains(got, "value") {
		t.Fatalf("expected body content, got:\n%s", got)
	}
}

func TestExplicitPrintBodyRendersImageOnTTY(t *testing.T) {
	data := pngBytes(t)
	c, out, _ := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	t.Setenv("RSH_IMAGE_PROTOCOL", "iterm2")
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(data)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", "https://api.example.com/logo.png"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\x1b]1337;File=") {
		t.Fatalf("expected terminal image output, got %q", got)
	}
	if bytes.Equal(out.Bytes(), data) {
		t.Fatalf("expected rendered image, got raw image bytes")
	}
}

func TestExplicitPrintRequestBodyModifiers(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"ok":true}`)
	if err := c.Run([]string{"restish", "post", "--rsh-print", "B", "https://api.example.com/items", "name: Alice"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != `{"name":"Alice"}` {
		t.Fatalf("request body = %q, want compact JSON", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "post", "--rsh-print", "Bpc", "https://api.example.com/items", "name: Alice"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected colored request body, got:\n%s", got)
	}
	stripped := stripANSI(got)
	if !strings.Contains(stripped, `"name": "Alice"`) {
		t.Fatalf("expected pretty request body, got:\n%s", stripped)
	}
}

func TestPrintRequestAndResponseTranscript(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	useJSONResponse(c, 200, `{"ok":true}`)
	if err := c.Run([]string{
		"restish", "post",
		"--rsh-header", "Authorization: Bearer secret",
		"--rsh-print", "HBhbp",
		"https://api.example.com/items",
		"name: Alice",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stripANSI(out.String())
	requireContains(t, got,
		"POST /items HTTP/1.1",
		"Host: api.example.com",
		"Authorization: <redacted>",
		`"name": "Alice"`,
		"HTTP/1.1 200 OK",
		`"ok": true`,
	)
}

func TestPrintRequestHeadersRedactsSensitiveQueryParams(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"ok":true}`)
	if err := c.Run([]string{"restish", "get", "--rsh-print", "H", "https://api.example.com/items?api_key=secret&token=abc&key=testing&auth=abc123def&page=1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stripANSI(out.String())
	if strings.Contains(got, "api_key=secret") || strings.Contains(got, "token=abc") || strings.Contains(got, "auth=abc123def") {
		t.Fatalf("printed request leaked sensitive query params:\n%s", got)
	}
	requireContains(t, got, "GET /items?", "api_key=%3Credacted%3E", "token=%3Credacted%3E", "auth=%3Credacted%3E", "key=testing", "page=1")
}

func TestExplicitAutoOutputFormatRedirectRendersBody(t *testing.T) {
	raw := `{"z":1}`
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, raw)
	if err := c.Run([]string{"restish", "get", "-o", "auto", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got == raw {
		t.Fatalf("explicit -o auto used raw redirected body path:\n%s", got)
	}
	if got, want := out.String(), "{\n  \"z\": 1\n}\n"; got != want {
		t.Fatalf("explicit -o auto = %q, want pretty rendered JSON body %q", got, want)
	}
}

func TestExplicitAutoOutputFormatRedirectRendersPlainTextBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useTextResponse(c, 200, "text/plain; charset=utf-8", "hello")
	if err := c.Run([]string{"restish", "get", "-o", "auto", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := out.String(), "hello\n"; got != want {
		t.Fatalf("explicit -o auto plain text = %q, want %q", got, want)
	}
}

func TestFilteredStructuredOutputPrettyByDefault(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"object":{"z":1,"items":[true,false]}}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.object", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := out.String(), "{\n  \"items\": [\n    true,\n    false\n  ],\n  \"z\": 1\n}\n"; got != want {
		t.Fatalf("filtered structured output = %q, want pretty JSON %q", got, want)
	}
}

func TestExplicitPrintBodyKeepsAutoOutputCompact(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"object":{"z":1}}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.object", "--rsh-print", "b", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := out.String(), "{\"z\":1}\n"; got != want {
		t.Fatalf("explicit --rsh-print=b output = %q, want compact JSON %q", got, want)
	}
}

func TestExplicitPrintBodyKeepsJSONFilterCompact(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"object":{"z":1}}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.object", "-o", "json", "--rsh-print", "b", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := out.String(), "{\"z\":1}\n"; got != want {
		t.Fatalf("explicit --rsh-print=b -o json output = %q, want compact JSON %q", got, want)
	}
}

func TestTTYTableOutputPrintsSingleHTTPPreamble(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	useJSONResponse(c, 200, `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`)
	if err := c.Run([]string{"restish", "get", "-o", "table", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stripANSI(out.String())
	if count := strings.Count(got, "HTTP/1.1 200 OK"); count != 1 {
		t.Fatalf("HTTP preamble count = %d, want 1:\n%s", count, got)
	}
	if !strings.Contains(got, "Alice") || !strings.Contains(got, "Bob") {
		t.Fatalf("table body missing rows:\n%s", got)
	}
}

// TestExitCode3xx verifies that a final 3xx response returns ExitCodeError{Code:3}.
func TestExitCode3xx(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useJSONResponse(c, 304, `{}`)
	err := c.Run([]string{"restish", "get", "https://api.example.com/items"})

	var exitErr *cli.ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 3 {
		t.Errorf("expected exit code 3, got %d", exitErr.Code)
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
	requested := false
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		requested = true
		return jsonResponse(200, `{}`), nil
	})
	err := c.Run([]string{"restish", "get", "-o", "nosuchformat", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected error for unknown output format, got nil")
	}
	if requested {
		t.Fatal("request should not be sent with unknown output format")
	}
}

func TestUnknownOutputFormatSuggestsNearestFormat(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent with unknown output format")
		return nil, nil
	})
	err := c.Run([]string{"restish", "get", "-o", "jsoon", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected error for unknown output format")
	}
	if !strings.Contains(err.Error(), `unknown output format "jsoon"`) ||
		!strings.Contains(err.Error(), `did you mean "json"?`) {
		t.Fatalf("unexpected unknown output format error: %v", err)
	}
}

// TestPrintResponseHeadersOnly verifies that --rsh-print h writes only
// response headers and no body.
func TestPrintResponseHeadersOnly(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Custom": []string{"hello"}},
			Body:       io.NopCloser(strings.NewReader(`{"key":"value"}`)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-print", "h", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stripANSI(out.String())
	requireContains(t, got, "HTTP/1.1 200 OK", "Content-Type: application/json", "X-Custom: hello")
	if strings.Contains(got, "key") || strings.Contains(got, "value") {
		t.Fatalf("body should not appear with --rsh-print h:\n%s", got)
	}
}

// TestPrintResponseHeadersAndBody verifies that --rsh-print hb writes headers
// then body, without request parts.
func TestPrintResponseHeadersAndBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 201, `{"created":true}`)
	if err := c.Run([]string{"restish", "post", "--rsh-print", "hb", "--rsh-ignore-status-code", "https://api.example.com/items", "name: test"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stripANSI(out.String())
	if !strings.Contains(got, "HTTP/1.1 201") {
		t.Fatalf("stdout missing response status:\n%s", got)
	}
	if !strings.Contains(got, "created") {
		t.Fatalf("stdout missing body:\n%s", got)
	}
	if strings.Contains(got, "POST") {
		t.Fatalf("request line should not appear with --rsh-print hb:\n%s", got)
	}
}

func TestPrintRejectsRawPart(t *testing.T) {
	c, _, _ := newTestCLI(t)
	requested := false
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		requested = true
		return jsonResponse(200, `{}`), nil
	})
	err := c.Run([]string{"restish", "get", "--rsh-print", "rb", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected --rsh-print raw part to be rejected")
	}
	if !strings.Contains(err.Error(), `unknown part "r"`) ||
		!strings.Contains(err.Error(), "H, B, h, b, p, c") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requested {
		t.Fatal("request should not be sent with invalid --rsh-print")
	}
}

func TestOutputValidationPreventsRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown output format",
			args: []string{"restish", "get", "-o", "nope", "https://api.example.com/items"},
			want: `unknown output format "nope"`,
		},
		{
			name: "unknown print part",
			args: []string{"restish", "get", "--rsh-print", "nope", "https://api.example.com/items"},
			want: `invalid --rsh-print value "nope"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requested := false
			c, _, _ := newTestCLI(t)
			c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				requested = true
				return jsonResponse(200, `{}`), nil
			})
			err := c.Run(tt.args)
			if err == nil {
				t.Fatalf("%v should fail", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected error: %v", err)
			}
			if requested {
				t.Fatalf("%v sent a request before output validation failed", tt.args)
			}
		})
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
	if got != "Alice" {
		t.Errorf("got %q, want %q", got, "Alice")
	}
}

func TestFilterArrayPreservesShapeByDefault(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"names":["Alice","Bob"]}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.names", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("expected JSON array output, got %q: %v", out.String(), err)
	}
	if len(got) != 2 || got[0] != "Alice" || got[1] != "Bob" {
		t.Fatalf("names = %#v, want Alice/Bob", got)
	}
}

func TestFilterLinesOutput(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"names":["Alice","Bob"]}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.names", "-o", "lines", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := out.String(), "Alice\nBob\n"; got != want {
		t.Fatalf("lines output = %q, want %q", got, want)
	}
}

func TestFilterLinesOutputRejectsObjects(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useJSONResponse(c, 200, `{"items":[{"name":"Alice"}]}`)
	err := c.Run([]string{"restish", "get", "-f", "body.items", "-o", "lines", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected -o lines to reject object arrays")
	}
	if !strings.Contains(err.Error(), "requires scalar values") {
		t.Fatalf("unexpected error: %v", err)
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

func TestFilterStatus(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 204, ``)
	if err := c.Run([]string{"restish", "get", "--rsh-status", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "204" {
		t.Fatalf("status output = %q, want 204", got)
	}
}

func TestFilterHeadersDoesNotReadBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	body := &failOnReadBody{t: t}
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Test": []string{"ok"}},
			Body:       body,
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-headers", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "X-Test") {
		t.Fatalf("headers output missing X-Test header:\n%s", got)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestFilterStatusDoesNotReadBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	body := &failOnReadBody{t: t}
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 204,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       body,
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-status", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "204" {
		t.Fatalf("status output = %q, want 204", got)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
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
	if got := strings.TrimSpace(out.String()); got != "Mon, 02 Jan 2006 15:04:05 GMT" {
		t.Fatalf("headers.Date output = %q", got)
	}
}

func TestFilterShorthandHeadersAllValue(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Set-Cookie":   []string{"session=secret", "theme=light"},
			},
			Body:    io.NopCloser(strings.NewReader(`{}`)),
			Request: r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-filter-lang", "shorthand", "-f", `{ct: headers_all.Content-Type[0], cookie: headers_all.Set-Cookie[1]}`, "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out.String(), err)
	}
	if decoded["ct"] != "application/json" || decoded["cookie"] != "theme=light" {
		t.Fatalf("unexpected headers_all filter output: %#v", decoded)
	}
}

func TestFilterShorthandCanProjectHeadersAndBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Server":       []string{"test-server"},
			},
			Body:    io.NopCloser(strings.NewReader(`{"integer":42}`)),
			Request: r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-filter-lang", "shorthand", "-f", `{int: body.integer, ct: headers.Server}`, "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out.String(), err)
	}
	if decoded["int"] != float64(42) || decoded["ct"] != "test-server" {
		t.Fatalf("unexpected filter output: %#v", decoded)
	}
}

func TestFilterJQCanProjectHeadersAndBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type":  []string{"application/json"},
				"Date":          []string{"Mon, 02 Jan 2006 15:04:05 GMT"},
				"X-Trace-Token": []string{"abc123"},
			},
			Body:    io.NopCloser(strings.NewReader(`{"name":"Alice"}`)),
			Request: r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-filter-lang", "jq", "-f", `{status: .status, date: .headers.Date, trace: .headers["X-Trace-Token"], name: .body.name}`, "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out.String(), err)
	}
	if decoded["status"] != float64(200) || decoded["date"] != "Mon, 02 Jan 2006 15:04:05 GMT" || decoded["trace"] != "abc123" || decoded["name"] != "Alice" {
		t.Fatalf("unexpected filter output: %#v", decoded)
	}
}

func TestFilterCanExposeSensitiveResponseHeaders(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header: http.Header{
				"Content-Type": []string{"application/json"},
				"Set-Cookie":   []string{"session=secret"},
				"X-Api-Key":    []string{"response-key"},
			},
			Body:    io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request: r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "-f", "@", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("expected JSON output, got %q: %v", out.String(), err)
	}
	headers, ok := decoded["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers = %#v", decoded["headers"])
	}
	if headers["Set-Cookie"] != "session=secret" || headers["X-Api-Key"] != "response-key" {
		t.Fatalf("sensitive filtered headers were not exposed: %#v", headers)
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

func TestJQLookingFilterWithoutBodyRootHints(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	useJSONResponse(c, 200, `[{"id":"one"},{"id":"two"}]`)
	if err := c.Run([]string{"restish", "get", "--rsh-collect", "-f", "map(.id)", "-o", "lines", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "null" {
		t.Fatalf("stdout = %q, want null", out.String())
	}
	requireContains(t, errOut.String(), "this looks like jq", ".body | map(.id)")
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

func TestStatusFlagWarnsWhenOverridingFilter(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	useJSONResponse(c, 201, `{"name":"Alice"}`)
	if err := c.Run([]string{"restish", "get", "-f", "body.name", "--rsh-status", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "201" {
		t.Fatalf("status output = %q, want 201", got)
	}
	if !strings.Contains(errOut.String(), "--rsh-status overrides -f") {
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

func TestExplicitPrintBodyImageNonTTYWritesOriginalBytes(t *testing.T) {
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
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", "https://api.example.com/logo.png"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("image bytes changed: got %d bytes, want %d", out.Len(), len(data))
	}
	if strings.HasPrefix(out.String(), `"`) {
		t.Fatalf("image body was JSON-encoded instead of written as bytes")
	}
}

func TestBinaryResponseDefaultNonTTYWritesOriginalBytes(t *testing.T) {
	data := []byte{0x00, 0x01, 0xff, 0x7f}
	for _, contentType := range []string{"application/octet-stream", "application/zip", "application/x-custom"} {
		t.Run(contentType, func(t *testing.T) {
			c, out, _ := newTestCLI(t)
			c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200,
					Proto:      "HTTP/1.1",
					Header:     http.Header{"Content-Type": []string{contentType}},
					Body:       io.NopCloser(bytes.NewReader(data)),
					Request:    r,
				}, nil
			})
			if err := c.Run([]string{"restish", "get", "https://api.example.com/file"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !bytes.Equal(out.Bytes(), data) {
				t.Fatalf("bytes changed: got %v, want %v", out.Bytes(), data)
			}
		})
	}
}

func TestExplicitPrintBodyBinaryNonTTYOmitNotice(t *testing.T) {
	data := []byte{0x00, 0x01, 0xff, 0x7f}
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			Body:       io.NopCloser(bytes.NewReader(data)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", "https://api.example.com/file"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Binary body omitted: 4 bytes (application/octet-stream)") {
		t.Fatalf("expected binary omit notice, got %q", got)
	}
	if strings.Contains(got, "AAH/fw==") {
		t.Fatalf("binary body was JSON base64-encoded:\n%s", got)
	}
}

func TestExplicitPrintBodyPlainTextNonTTYStaysText(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useTextResponse(c, 200, "text/plain; charset=utf-8", "hello")
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", "https://api.example.com/message"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := out.String(), "hello\n"; got != want {
		t.Fatalf("plain text body-only output = %q, want %q", got, want)
	}
}

func TestExplicitPrintBodyNoContentNonTTYWritesNothing(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 204, "")
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", "https://api.example.com/empty"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got != "" {
		t.Fatalf("no-content body-only output = %q, want empty", got)
	}
}

func TestExplicitPrintBodyJSONNullNonTTYWritesNull(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, "null")
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", "https://api.example.com/null"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := out.String(), "null\n"; got != want {
		t.Fatalf("JSON null body-only output = %q, want %q", got, want)
	}
}

func TestStructuredJSONResponseDefaultNonTTYPreservesOriginalBytes(t *testing.T) {
	raw := "{\n  \"ok\": true\n}\n"
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, raw)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/status"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got != raw {
		t.Fatalf("default redirected output changed body bytes:\ngot  %q\nwant %q", got, raw)
	}
}

func TestMalformedJSONDefaultNonTTYPreservesOriginalBytes(t *testing.T) {
	raw := "{not json"
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, raw)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/status"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got != raw {
		t.Fatalf("default redirected output changed malformed JSON bytes:\ngot  %q\nwant %q", got, raw)
	}
}

func TestOutputFormatAutoEnvPreservesDefaultRawRedirect(t *testing.T) {
	t.Setenv("RSH_OUTPUT_FORMAT", "auto")
	raw := `{"z":1}`
	c, out, _ := newTestCLI(t)
	useJSONResponse(c, 200, raw)
	if err := c.Run([]string{"restish", "get", "https://api.example.com/status"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got != raw {
		t.Fatalf("RSH_OUTPUT_FORMAT=auto changed redirected body bytes:\ngot  %q\nwant %q", got, raw)
	}
}

func TestMalformedCBORDefaultNonTTYPreservesOriginalBytes(t *testing.T) {
	raw := []byte{0xff}
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/cbor"}},
			Body:       io.NopCloser(bytes.NewReader(raw)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/status"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("default redirected output changed malformed CBOR bytes:\ngot  %x\nwant %x", out.Bytes(), raw)
	}
}

func TestStructuredCBORResponseDefaultNonTTYPreservesOriginalBytes(t *testing.T) {
	c, out, _ := newTestCLI(t)
	raw := useCBORResponse(t, c, 200, map[string]any{"ok": true})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/status"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(out.Bytes(), raw) {
		t.Fatalf("default redirected output changed CBOR bytes:\ngot  %x\nwant %x", out.Bytes(), raw)
	}
}

func TestExplicitJSONOutputTranscodesStructuredCBOR(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useCBORResponse(t, c, 200, map[string]any{"ok": true})
	if err := c.Run([]string{"restish", "get", "-o", "json", "https://api.example.com/status"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]bool
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("explicit JSON output is not JSON: %q: %v", out.String(), err)
	}
	if !got["ok"] {
		t.Fatalf("explicit JSON output = %#v, want ok=true", got)
	}
}

type failOnReadBody struct {
	t      *testing.T
	closed bool
}

func (b *failOnReadBody) Read([]byte) (int, error) {
	b.t.Fatal("response body was read for body-free --rsh-print output")
	return 0, io.EOF
}

func (b *failOnReadBody) Close() error {
	b.closed = true
	return nil
}

func TestPrintResponseHeadersOnlyDoesNotReadBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	body := &failOnReadBody{t: t}
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Test": []string{"ok"}},
			Body:       body,
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "--rsh-print", "h", "https://api.example.com/status"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stripANSI(out.String())
	if !strings.Contains(got, "HTTP/1.1 200 OK") || !strings.Contains(got, "X-Test: ok") {
		t.Fatalf("headers-only output missing response metadata:\n%s", got)
	}
	if strings.Contains(got, "body") {
		t.Fatalf("headers-only output included body data:\n%s", got)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}
