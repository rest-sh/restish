package output_test

import (
	"bytes"
	"encoding/json"
	"image/color"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/content"
	"github.com/danielgtaylor/restish/v2/internal/output"
)

var testRegistry = content.Default()

// makeResp builds a minimal *http.Response for use in tests.
func makeResp(status int, contentType, body string) *http.Response {
	var bodyReader io.ReadCloser
	if body != "" {
		bodyReader = io.NopCloser(strings.NewReader(body))
	} else {
		bodyReader = http.NoBody
	}
	header := http.Header{}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		Proto:      "HTTP/1.1",
		StatusCode: status,
		Header:     header,
		Body:       bodyReader,
	}
}

// --- Normalize ---

func TestNormalize_JSONBody(t *testing.T) {
	resp := makeResp(200, "application/json", `{"name":"Alice","age":30}`)
	r, err := output.Normalize(resp, testRegistry, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := r.Body.(map[string]any)
	if !ok {
		t.Fatalf("expected map body, got %T", r.Body)
	}
	if m["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", m["name"])
	}
}

func TestNormalize_NonJSONBody(t *testing.T) {
	resp := makeResp(200, "text/plain", "hello world")
	r, err := output.Normalize(resp, testRegistry, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Body != "hello world" {
		t.Errorf("expected string body, got %v", r.Body)
	}
}

func TestNormalize_EmptyBody(t *testing.T) {
	resp := makeResp(204, "", "")
	r, err := output.Normalize(resp, testRegistry, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Body != nil {
		t.Errorf("expected nil body for empty response, got %v", r.Body)
	}
}

func TestNormalize_Status(t *testing.T) {
	resp := makeResp(404, "application/json", `{}`)
	r, err := output.Normalize(resp, testRegistry, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status != 404 {
		t.Errorf("expected status 404, got %d", r.Status)
	}
}

func TestNormalize_HeadersCanonicalized(t *testing.T) {
	resp := makeResp(200, "application/json", `{}`)
	// Go's http package canonicalizes keys automatically; verify they arrive
	// in the Response using the canonical (title-case) form.
	resp.Header.Set("x-custom-header", "testval")
	r, err := output.Normalize(resp, testRegistry, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Headers["X-Custom-Header"] != "testval" {
		t.Errorf("expected X-Custom-Header=testval, got %q", r.Headers["X-Custom-Header"])
	}
}

func TestNormalize_Proto(t *testing.T) {
	resp := makeResp(200, "application/json", `{}`)
	resp.Proto = "HTTP/2.0"
	r, _ := output.Normalize(resp, testRegistry, 0)
	if r.Proto != "HTTP/2.0" {
		t.Errorf("expected proto HTTP/2.0, got %q", r.Proto)
	}
}

// --- StatusToExitCode ---

func TestStatusToExitCode(t *testing.T) {
	cases := []struct {
		status   int
		wantCode int
	}{
		{200, 0},
		{201, 0},
		{204, 0},
		{301, 3},
		{302, 3},
		{400, 4},
		{401, 4},
		{404, 4},
		{500, 5},
		{503, 5},
	}
	for _, tc := range cases {
		got := output.StatusToExitCode(tc.status)
		if got != tc.wantCode {
			t.Errorf("StatusToExitCode(%d) = %d, want %d", tc.status, got, tc.wantCode)
		}
	}
}

// --- JSONFormatter ---

func TestJSONFormatter_OutputsBody(t *testing.T) {
	resp := &output.Response{
		Proto:  "HTTP/1.1",
		Status: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: map[string]any{"key": "value"},
	}

	var buf bytes.Buffer
	f := output.DefaultFormatters()["json"]
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output must be valid JSON.
	var v any
	if err := json.Unmarshal(buf.Bytes(), &v); err != nil {
		t.Errorf("json formatter produced invalid JSON: %v\noutput: %s", err, buf.String())
	}

	// Output is the body only (no status, no headers).
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %T", v)
	}
	if m["key"] != "value" {
		t.Errorf("expected key=value, got %v", m["key"])
	}
	// Must NOT contain the status code at the top level.
	if _, hasStatus := m["status"]; hasStatus {
		t.Error("json formatter should output body only, not the full response")
	}
}

func TestJSONFormatter_NilBodyOutputsNull(t *testing.T) {
	resp := &output.Response{Status: 204, Headers: map[string]string{}}
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["json"].Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "null" {
		t.Errorf("expected 'null' for nil body, got %q", buf.String())
	}
}

// --- ReadableFormatter ---

func TestReadableFormatter_ContainsStatus(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    map[string]any{"hello": "world"},
	}

	var buf bytes.Buffer
	f := output.DefaultFormatters()["readable"]
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, "200") {
		t.Errorf("readable output missing status code:\n%s", got)
	}
	if !strings.Contains(got, "Content-Type") {
		t.Errorf("readable output missing Content-Type header:\n%s", got)
	}
}

func TestReadableFormatter_BodyIsValidJSON(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    map[string]any{"name": "Alice", "age": float64(30)},
	}

	var buf bytes.Buffer
	f := output.DefaultFormatters()["readable"]
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The readable output is: status line + headers + blank line + JSON body.
	// Find the blank line that separates headers from body.
	parts := strings.SplitN(buf.String(), "\n\n", 2)
	if len(parts) != 2 {
		t.Fatalf("expected blank line separator in readable output:\n%s", buf.String())
	}
	bodyPart := strings.TrimSpace(parts[1])

	var v any
	if err := json.Unmarshal([]byte(bodyPart), &v); err != nil {
		t.Errorf("body part of readable output is not valid JSON: %v\nbody: %s", err, bodyPart)
	}
}

func TestReadableFormatter_NilBodyNoBody(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  204,
		Headers: map[string]string{},
	}
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["readable"].Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have status line but no body content after the blank line.
	if !strings.Contains(buf.String(), "204") {
		t.Errorf("expected 204 in readable output: %q", buf.String())
	}
}

func TestReadableFormatter_ImageBodyIncludesHeadersAndRenderedImage(t *testing.T) {
	clearGraphicsEnv(t)

	data := makePNG(t, 4, 4, color.RGBA{255, 0, 0, 255})
	resp := &output.Response{
		Proto:  "HTTP/1.1",
		Status: 200,
		Headers: map[string]string{
			"Content-Type": "image/png",
			"X-Test":       "present",
		},
		Body: string(data),
		Raw:  data,
	}

	var buf bytes.Buffer
	if err := output.DefaultFormatters()["readable"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Content-Type") {
		t.Errorf("expected readable image output to include headers")
	}
	if !strings.Contains(got, "X-Test") {
		t.Errorf("expected readable image output to include custom headers")
	}
	if !strings.Contains(got, "▀") {
		t.Errorf("expected readable image output to render the image inline")
	}
}

// --- Select ---

func TestSelect_TTYDefaultsToReadable(t *testing.T) {
	fmts := output.DefaultFormatters()
	f, ok := output.Select(fmts, "", true)
	if !ok {
		t.Fatal("Select returned !ok")
	}
	_, isReadable := f.(*output.ReadableFormatter)
	if !isReadable {
		t.Errorf("expected ReadableFormatter for TTY, got %T", f)
	}
}

func TestSelect_NonTTYDefaultsToRaw(t *testing.T) {
	fmts := output.DefaultFormatters()
	f, ok := output.Select(fmts, "", false)
	if !ok {
		t.Fatal("Select returned !ok")
	}
	_, isRaw := f.(*output.RawFormatter)
	if !isRaw {
		t.Errorf("expected RawFormatter for non-TTY, got %T", f)
	}
}

func TestSelect_ExplicitFormat(t *testing.T) {
	fmts := output.DefaultFormatters()
	f, ok := output.Select(fmts, "readable", false) // non-TTY but explicit readable
	if !ok {
		t.Fatal("Select returned !ok")
	}
	_, isReadable := f.(*output.ReadableFormatter)
	if !isReadable {
		t.Errorf("expected ReadableFormatter, got %T", f)
	}
}

func TestSelect_UnknownFormat(t *testing.T) {
	fmts := output.DefaultFormatters()
	_, ok := output.Select(fmts, "nosuchformat", false)
	if ok {
		t.Error("expected ok=false for unknown format name")
	}
}
