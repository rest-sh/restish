package output_test

import (
	"bytes"
	"encoding/json"
	"image/color"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/content"
	"github.com/rest-sh/restish/v2/internal/output"
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

func TestNormalize_BodylessResponsesSkipContentEncoding(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		method   string
		encoding string
	}{
		{name: "informational gzip", status: 103, encoding: "gzip"},
		{name: "no content gzip", status: http.StatusNoContent, encoding: "gzip"},
		{name: "reset content br", status: http.StatusResetContent, encoding: "br"},
		{name: "not modified gzip", status: http.StatusNotModified, encoding: "gzip"},
		{name: "head success gzip", status: http.StatusOK, method: http.MethodHead, encoding: "gzip"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := makeResp(tt.status, "application/json", "not a compressed body")
			resp.Header.Set("Content-Encoding", tt.encoding)
			if tt.method != "" {
				resp.Request = &http.Request{Method: tt.method}
			}
			r, err := output.Normalize(resp, testRegistry, 0)
			if err != nil {
				t.Fatalf("Normalize: %v", err)
			}
			if r.Body != nil || len(r.Raw) != 0 {
				t.Fatalf("body = %#v raw = %q, want no body", r.Body, r.Raw)
			}
		})
	}
}

func TestNormalize_EncodedEmptyOKStillErrors(t *testing.T) {
	resp := makeResp(http.StatusOK, "application/json", "")
	resp.Header.Set("Content-Encoding", "gzip")
	_, err := output.Normalize(resp, testRegistry, 0)
	if err == nil {
		t.Fatal("expected empty gzip body on 200 OK to fail")
	}
	if !strings.Contains(err.Error(), "decompressing response") {
		t.Fatalf("error = %v, want decompression error", err)
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
	if output.Header(r.Headers, "X-Custom-Header") != "testval" {
		t.Errorf("expected X-Custom-Header=testval, got %q", r.Headers["X-Custom-Header"])
	}
}

func TestNormalize_PreservesRepeatedHeaders(t *testing.T) {
	resp := makeResp(200, "application/json", `{}`)
	resp.Header.Add("Set-Cookie", "a=1")
	resp.Header.Add("Set-Cookie", "b=2")
	resp.Header.Add("Warning", `199 example "old"`)
	resp.Header.Add("Warning", `299 example "new"`)

	r, err := output.Normalize(resp, testRegistry, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := output.HeaderValues(r.Headers, "Set-Cookie"); len(got) != 2 || got[0] != "a=1" || got[1] != "b=2" {
		t.Fatalf("Set-Cookie values = %#v", got)
	}
	if got := output.HeaderValues(r.Headers, "Warning"); len(got) != 2 || got[0] != `199 example "old"` || got[1] != `299 example "new"` {
		t.Fatalf("Warning values = %#v", got)
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
		{101, 1},
		{600, 1},
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
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
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
	resp := &output.Response{Status: 204, Headers: map[string][]string{}}
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["json"].Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "null" {
		t.Errorf("expected 'null' for nil body, got %q", buf.String())
	}
}

func TestYAMLFormatter_OutputsBodyWithoutPreamble(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/2.0",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    map[string]any{"key": "value"},
	}

	var buf bytes.Buffer
	if err := output.DefaultFormatters()["yaml"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "HTTP/2.0") || strings.Contains(got, "Content-Type") {
		t.Fatalf("yaml formatter should output body only, got:\n%s", got)
	}
	if !strings.Contains(got, "key: value") {
		t.Fatalf("yaml formatter missing body value:\n%s", got)
	}
}

func TestJSONFormatter_DoesNotEscapeHTML(t *testing.T) {
	resp := &output.Response{
		Body: map[string]any{"url": "https://api.example.com?a=1&b=2"},
	}
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["json"].Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "&") {
		t.Fatalf("expected ampersand to remain unescaped, got %q", buf.String())
	}
	if strings.Contains(buf.String(), `\u0026`) {
		t.Fatalf("expected HTML escaping to be disabled, got %q", buf.String())
	}
}

func TestJSONFormatter_WithColorHighlightsBodyOnly(t *testing.T) {
	resp := &output.Response{
		Proto:  "HTTP/1.1",
		Status: 200,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: map[string]any{"colored": true},
	}

	var buf bytes.Buffer
	if err := output.DefaultFormatters()["json"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("expected ANSI highlighting, got %q", buf.String())
	}

	stripped := stripANSI(buf.String())
	if strings.Contains(stripped, "HTTP/1.1") || strings.Contains(stripped, "Content-Type") {
		t.Fatalf("json formatter should output body only, got:\n%s", stripped)
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stripped)), new(any)); err != nil {
		t.Fatalf("colored JSON was not valid after stripping ANSI: %v\n%s", err, stripped)
	}
}

func TestNDJSONFormatter_OutputsOneValuePerLine(t *testing.T) {
	resp := &output.Response{
		Body: []any{
			map[string]any{"id": 1},
			map[string]any{"id": 2},
		},
	}

	var buf bytes.Buffer
	f := output.DefaultFormatters()["ndjson"]
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d: %q", len(lines), buf.String())
	}
	for i, line := range lines {
		var item map[string]int
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("line %d is not valid JSON: %q: %v", i+1, line, err)
		}
		if item["id"] != i+1 {
			t.Fatalf("line %d id = %d, want %d", i+1, item["id"], i+1)
		}
	}
}

func TestNDJSONFormatter_WithColorHighlightsEachLine(t *testing.T) {
	resp := &output.Response{
		Body: []any{
			map[string]any{"id": 1},
			map[string]any{"id": 2},
		},
	}

	var buf bytes.Buffer
	if err := output.DefaultFormatters()["ndjson"].Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("expected ANSI highlighting, got %q", buf.String())
	}

	stripped := strings.TrimSpace(stripANSI(buf.String()))
	lines := strings.Split(stripped, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines after stripping ANSI, got %d: %q", len(lines), stripped)
	}
	for i, line := range lines {
		var item map[string]int
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("line %d is not valid JSON after stripping ANSI: %q: %v", i+1, line, err)
		}
		if item["id"] != i+1 {
			t.Fatalf("line %d id = %d, want %d", i+1, item["id"], i+1)
		}
	}
}

func TestNDJSONFormatter_StreamWithColorHighlightsEachValue(t *testing.T) {
	formatter := output.DefaultFormatters()["ndjson"].(output.ValueStreamFormatter)
	var buf bytes.Buffer
	stream, err := formatter.StartValueStream(&buf, &output.Response{}, true)
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 1}); err != nil {
		t.Fatalf("write first value: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 2}); err != nil {
		t.Fatalf("write second value: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	if !strings.Contains(buf.String(), "\x1b[") {
		t.Fatalf("expected ANSI highlighting, got %q", buf.String())
	}

	stripped := strings.TrimSpace(stripANSI(buf.String()))
	lines := strings.Split(stripped, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines after stripping ANSI, got %d: %q", len(lines), stripped)
	}
}

func TestLinesFormatter_OutputsScalarArrayOneValuePerLine(t *testing.T) {
	resp := &output.Response{Body: []any{"Alice", "Bob", float64(3), true, nil}}
	var buf bytes.Buffer
	if err := output.DefaultFormatters()["lines"].Format(&buf, resp, false); err != nil {
		t.Fatalf("lines formatter: %v", err)
	}
	if got, want := buf.String(), "Alice\nBob\n3\ntrue\nnull\n"; got != want {
		t.Fatalf("lines output = %q, want %q", got, want)
	}
}

func TestLinesFormatter_RejectsStructuredValue(t *testing.T) {
	resp := &output.Response{Body: map[string]any{"name": "Alice"}}
	var buf bytes.Buffer
	err := output.DefaultFormatters()["lines"].Format(&buf, resp, false)
	if err == nil {
		t.Fatal("expected lines formatter to reject structured value")
	}
	if !strings.Contains(err.Error(), "requires scalar values") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- AutoFormatter ---

func TestAutoFormatter_DoesNotPrintHTTPPreamble(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    map[string]any{"hello": "world"},
	}

	var buf bytes.Buffer
	f := &output.AutoFormatter{}
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()

	if strings.Contains(got, "HTTP/1.1") || strings.Contains(got, "Content-Type") {
		t.Errorf("auto output included HTTP preamble:\n%s", got)
	}
	if !strings.Contains(got, `"hello": "world"`) {
		t.Errorf("auto output missing body:\n%s", got)
	}
}

func TestAutoFormatter_BodyIsValidJSON(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    map[string]any{"name": "Alice", "age": float64(30)},
	}

	var buf bytes.Buffer
	f := &output.AutoFormatter{}
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var v any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &v); err != nil {
		t.Errorf("auto output is not valid JSON: %v\nbody: %s", err, buf.String())
	}
}

func TestAutoFormatter_PrintsPlainTextBody(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"text/plain"}},
		Body:    "hello & goodbye",
		Raw:     []byte("hello & goodbye"),
	}
	var buf bytes.Buffer
	if err := (&output.AutoFormatter{}).Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "hello & goodbye" {
		t.Fatalf("expected plain text body, got %q", buf.String())
	}
}

func TestAutoFormatter_OmitsBinaryBody(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/octet-stream"}},
		Body:    []byte{0, 1, 2, 3},
		Raw:     []byte{0, 1, 2, 3},
	}
	var buf bytes.Buffer
	if err := (&output.AutoFormatter{}).Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "AAECAw==") {
		t.Fatalf("expected binary body to be omitted, got base64 JSON output:\n%s", got)
	}
	for _, want := range []string{"Binary body omitted: 4 bytes", "application/octet-stream", "Redirect stdout"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in auto binary notice, got:\n%s", want, got)
		}
	}
}

func TestAutoFormatter_NilBodyNoBody(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  204,
		Headers: map[string][]string{},
	}
	var buf bytes.Buffer
	if err := (&output.AutoFormatter{}).Format(&buf, resp, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("expected no body output, got: %q", buf.String())
	}
}

func TestAutoFormatter_FramedValueStreamRootArray(t *testing.T) {
	var buf bytes.Buffer
	stream, err := (&output.AutoFormatter{}).StartFramedValueStream(&buf, &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
	}, false, output.FramedValueTemplate{
		ItemIndent:  "  ",
		CloseIndent: "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 1}); err != nil {
		t.Fatalf("WriteValue: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 2}); err != nil {
		t.Fatalf("WriteValue: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := buf.String()
	body := strings.TrimSpace(got)
	var parsed []map[string]int
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("expected framed auto stream to be valid JSON, got %q: %v", body, err)
	}
	if len(parsed) != 2 || parsed[0]["id"] != 1 || parsed[1]["id"] != 2 {
		t.Fatalf("unexpected parsed body: %#v", parsed)
	}
}

func TestAutoFormatter_FramedValueStreamWrappedObject(t *testing.T) {
	var buf bytes.Buffer
	stream, err := (&output.AutoFormatter{}).StartFramedValueStream(&buf, &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
	}, false, output.FramedValueTemplate{
		Prefix:      "{\n  \"data\": ",
		Suffix:      ",\n  \"meta\": {\n    \"source\": \"test\"\n  }\n}",
		ItemIndent:  "    ",
		CloseIndent: "  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 1}); err != nil {
		t.Fatalf("WriteValue: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 2}); err != nil {
		t.Fatalf("WriteValue: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := buf.String()
	body := strings.TrimSpace(got)
	var parsed struct {
		Data []map[string]int  `json:"data"`
		Meta map[string]string `json:"meta"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("expected wrapped framed auto stream to be valid JSON, got %q: %v", body, err)
	}
	if len(parsed.Data) != 2 || parsed.Data[0]["id"] != 1 || parsed.Data[1]["id"] != 2 {
		t.Fatalf("unexpected parsed data: %#v", parsed.Data)
	}
	if parsed.Meta["source"] != "test" {
		t.Fatalf("unexpected meta: %#v", parsed.Meta)
	}
}

func TestAutoFormatter_ImageBodyRendersImageWithoutHeaders(t *testing.T) {
	clearGraphicsEnv(t)

	data := makePNG(t, 4, 4, color.RGBA{255, 0, 0, 255})
	resp := &output.Response{
		Proto:  "HTTP/1.1",
		Status: 200,
		Headers: map[string][]string{
			"Content-Type": {"image/png"},
			"X-Test":       {"present"},
		},
		Body: string(data),
		Raw:  data,
	}

	var buf bytes.Buffer
	if err := (&output.AutoFormatter{}).Format(&buf, resp, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "Content-Type") || strings.Contains(got, "X-Test") {
		t.Errorf("expected auto image output to omit headers")
	}
	if !strings.Contains(got, "▀") {
		t.Errorf("expected auto image output to render the image inline")
	}
}
