package cli_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/internal/cli"
)

// sseBody builds an SSE response body from the provided event data strings.
func sseBody(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		fmt.Fprintf(&b, "data: %s\n\n", e)
	}
	return b.String()
}

func gzipBytes(t *testing.T, data string) []byte {
	t.Helper()
	var encoded bytes.Buffer
	gz := gzip.NewWriter(&encoded)
	if _, err := gz.Write([]byte(data)); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return encoded.Bytes()
}

func newStreamApp(t *testing.T, contentType, body string) *testApp {
	t.Helper()
	app := newTestApp(t)
	app.UseTextResponse(http.StatusOK, contentType, body)
	return app
}

func runStream(t *testing.T, contentType, body string, path string, args ...string) *testApp {
	t.Helper()
	app := newStreamApp(t, contentType, body)
	app.Run(append([]string{"get", "https://api.example.com" + path}, args...)...)
	return app
}

// TestSSEThreeEvents verifies that redirected, untransformed SSE output
// preserves raw event-stream framing.
func TestSSEThreeEvents(t *testing.T) {
	want := sseBody(`{"n":1}`, `{"n":2}`, `{"n":3}`)
	if got := runStream(t, "text/event-stream", want, "/events").Stdout.String(); got != want {
		t.Fatalf("raw SSE output = %q, want %q", got, want)
	}
}

func TestSSEPrintAutoPreservesRawRedirectedStream(t *testing.T) {
	tests := []struct {
		name string
		env  bool
		args []string
	}{
		{name: "env", env: true},
		{name: "flag", args: []string{"--rsh-print", "auto"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env {
				t.Setenv("RSH_PRINT", "auto")
			}
			want := sseBody(`{"n":1}`, `{"n":2}`)
			got := runStream(t, "text/event-stream", want, "/events", tt.args...).Stdout.String()
			if got != want {
				t.Fatalf("auto print output = %q, want raw stream %q", got, want)
			}
		})
	}
}

func TestStreamingRawRedirectDecompressesContentEncoding(t *testing.T) {
	raw := "{\"n\":1}\n{\"n\":2}\n"
	encoded := gzipBytes(t, raw)

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(encoded)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := out.String(); got != raw {
		t.Fatalf("raw streaming output = %q, want decompressed %q", got, raw)
	}
}

func TestStreamingRenderedOutputDecompressesContentEncoding(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		path        string
	}{
		{name: "sse", contentType: "text/event-stream", body: sseBody(`{"n":1}`, `{"n":2}`), path: "/events"},
		{name: "ndjson", contentType: "application/x-ndjson", body: "{\"n\":1}\n{\"n\":2}\n", path: "/stream"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := gzipBytes(t, tt.body)
			app := newTestApp(t)
			app.UseTransport(func(r *http.Request) (*http.Response, error) {
				resp := textResponse(http.StatusOK, tt.contentType, string(encoded), r)
				resp.Header.Set("Content-Encoding", "gzip")
				return resp, nil
			})
			app.Run("get", "-o", "ndjson", "https://api.example.com"+tt.path)
			if got, want := app.Stdout.String(), "{\"n\":1}\n{\"n\":2}\n"; got != want {
				t.Fatalf("rendered compressed stream output = %q, want %q", got, want)
			}
		})
	}
}

func TestStreamingHeadersOnlyAllowsJSONOutputFormat(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		path        string
	}{
		{name: "sse", contentType: "text/event-stream", body: sseBody(`{"n":1}`), path: "/events"},
		{name: "ndjson", contentType: "application/x-ndjson", body: "{\"n\":1}\n", path: "/stream"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newStreamApp(t, tt.contentType, tt.body)
			app.UseTransport(func(r *http.Request) (*http.Response, error) {
				resp := textResponse(http.StatusOK, tt.contentType, tt.body, r)
				resp.Header.Set("X-Test", "ok")
				return resp, nil
			})
			app.Run("get", "--rsh-print", "h", "-o", "json", "https://api.example.com"+tt.path)
			got := stripANSI(app.Stdout.String())
			if !strings.Contains(got, "HTTP/1.1 200 OK") || !strings.Contains(got, "X-Test: ok") {
				t.Fatalf("headers-only output missing response metadata:\n%s", got)
			}
			if strings.Contains(got, `"n"`) || strings.Contains(got, "data:") {
				t.Fatalf("headers-only output included stream body:\n%s", got)
			}
		})
	}
}

func TestMislabeledJSONLinesHeadersOnlyDoesNotReadBody(t *testing.T) {
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
	if err := c.Run([]string{"restish", "get", "--rsh-max-items", "1", "--rsh-print", "h", "https://api.example.com/stream"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := stripANSI(out.String())
	if !strings.Contains(got, "HTTP/1.1 200 OK") || !strings.Contains(got, "X-Test: ok") {
		t.Fatalf("headers-only output missing response metadata:\n%s", got)
	}
	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestSSEPrintRequestHeadersAppearsBeforeStreamOutput(t *testing.T) {
	app := runStream(t, "text/event-stream", sseBody(`{"n":1}`, `{"n":2}`), "/events", "--rsh-print", "Hhb")
	got := stripANSI(app.Stdout.String())
	reqIdx := strings.Index(got, "GET /events")
	respIdx := strings.Index(got, "HTTP/1.1 200")
	bodyIdx := strings.Index(got, `"n":1`)
	if reqIdx < 0 {
		t.Fatalf("request line not found:\n%s", got)
	}
	if respIdx < 0 {
		t.Fatalf("response status line not found:\n%s", got)
	}
	if bodyIdx < 0 {
		t.Fatalf("stream body not found:\n%s", got)
	}
	if reqIdx > respIdx || respIdx > bodyIdx {
		t.Fatalf("expected request headers before response headers before body:\n%s", got)
	}
}

func TestNDJSONPrintRequestHeadersAppearsBeforeStreamOutput(t *testing.T) {
	app := runStream(t, "application/x-ndjson", "{\"n\":1}\n{\"n\":2}\n", "/stream", "--rsh-print", "hb")
	got := stripANSI(app.Stdout.String())
	if !strings.Contains(got, "HTTP/1.1 200") {
		t.Fatalf("response status not found:\n%s", got)
	}
	if !strings.Contains(got, `"n":1`) {
		t.Fatalf("stream body not found:\n%s", got)
	}
	headerIdx := strings.Index(got, "HTTP/1.1 200")
	bodyIdx := strings.Index(got, `"n":1`)
	if headerIdx > bodyIdx {
		t.Fatalf("response headers should appear before stream body:\n%s", got)
	}
}

func TestSSEPrintHeadersOnlyOmitsStreamBody(t *testing.T) {
	app := runStream(t, "text/event-stream", sseBody(`{"n":1}`, `{"n":2}`), "/events", "--rsh-print", "h")
	got := stripANSI(app.Stdout.String())
	if !strings.Contains(got, "HTTP/1.1 200") {
		t.Fatalf("response status not found:\n%s", got)
	}
	if strings.Contains(got, `"n"`) || strings.Contains(got, "data:") {
		t.Fatalf("headers-only stream output included body data:\n%s", got)
	}
}

func TestNDJSONPrintRequestHeadersOnlyOmitsStreamBody(t *testing.T) {
	app := runStream(t, "application/x-ndjson", "{\"n\":1}\n{\"n\":2}\n", "/stream", "--rsh-print", "H")
	got := stripANSI(app.Stdout.String())
	if !strings.Contains(got, "GET /stream HTTP/1.1") {
		t.Fatalf("request line not found:\n%s", got)
	}
	if strings.Contains(got, "HTTP/1.1 200") || strings.Contains(got, `"n"`) {
		t.Fatalf("request-headers-only stream output included response data:\n%s", got)
	}
}

func TestSSEDataFieldWithoutColonPreservesBlankLine(t *testing.T) {
	app := runStream(t, "text/event-stream", "data: first\ndata\ndata: third\n\n", "/events", "-f", "body.data", "-o", "lines")
	if got, want := app.Stdout.String(), "first\n\nthird\n"; got != want {
		t.Fatalf("SSE data output = %q, want %q", got, want)
	}
}

// TestNDJSONThreeLines verifies that redirected, untransformed NDJSON output
// preserves raw records.
func TestNDJSONThreeLines(t *testing.T) {
	want := "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n"
	if got := runStream(t, "application/x-ndjson", want, "/stream").Stdout.String(); got != want {
		t.Fatalf("raw NDJSON output = %q, want %q", got, want)
	}
}

func TestNDJSONLineLimitUsesMaxBodySize(t *testing.T) {
	message := strings.Repeat("x", 1200*1024)
	line := `{"message":"` + message + `"}`
	app := runStream(t, "application/x-ndjson", line+"\n", "/stream", "--rsh-max-body-size", "2", "-f", "body.message", "-o", "lines")
	if got := strings.TrimSpace(app.Stdout.String()); got != message {
		t.Fatalf("large NDJSON message length = %d, want %d", len(got), len(message))
	}
}

func TestNDJSONFlushesBeforeResponseCompletes(t *testing.T) {
	firstLineWritten := make(chan struct{})
	finishResponse := make(chan struct{})
	var finishOnce sync.Once
	finish := func() {
		finishOnce.Do(func() { close(finishResponse) })
	}
	defer finish()

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not implement http.Flusher")
		}
		fmt.Fprintln(w, `{"n":1}`)
		flusher.Flush()
		close(firstLineWritten)
		<-finishResponse
		fmt.Fprintln(w, `{"n":2}`)
		flusher.Flush()
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	out := &signalWriter{needle: []byte(`"n": 1`), seen: make(chan struct{})}
	c, _, _ := newTestCLI(t)
	c.Stdout = out
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run([]string{"restish", "get", "--rsh-print", "bp", srv.URL + "/stream"})
	}()

	select {
	case <-firstLineWritten:
	case <-time.After(time.Second):
		t.Fatal("server did not write first stream line")
	}

	select {
	case <-out.seen:
	case err := <-errCh:
		t.Fatalf("command finished before response completed: %v", err)
	case <-time.After(time.Second):
		t.Fatal("first NDJSON line was not rendered before response completed")
	}

	finish()
	if err := <-errCh; err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := out.String(); !strings.Contains(got, `"n": 2`) {
		t.Fatalf("expected second line after response completed, got:\n%s", got)
	}
}

func TestNDJSONTimeoutOnlyBoundsHeaders(t *testing.T) {
	secondLine := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not implement http.Flusher")
		}
		fmt.Fprintln(w, `{"n":1}`)
		flusher.Flush()
		time.Sleep(75 * time.Millisecond)
		fmt.Fprintln(w, `{"n":2}`)
		flusher.Flush()
		close(secondLine)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", "--rsh-timeout", "25ms", "--rsh-max-items", "2", srv.URL + "/stream"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	select {
	case <-secondLine:
	default:
		t.Fatal("server did not write second stream line")
	}
	got := out.String()
	if !strings.Contains(got, `"n": 1`) || !strings.Contains(got, `"n": 2`) {
		t.Fatalf("expected both stream lines after timeout elapsed, got:\n%s", got)
	}
}

func TestSSEFlushesBeforeResponseCompletes(t *testing.T) {
	firstEventWritten := make(chan struct{})
	finishResponse := make(chan struct{})
	var finishOnce sync.Once
	finish := func() {
		finishOnce.Do(func() { close(finishResponse) })
	}
	defer finish()

	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not implement http.Flusher")
		}
		fmt.Fprint(w, "data: {\"n\":1}\n\n")
		flusher.Flush()
		close(firstEventWritten)
		<-finishResponse
		fmt.Fprint(w, "data: {\"n\":2}\n\n")
		flusher.Flush()
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	out := &signalWriter{needle: []byte(`"n": 1`), seen: make(chan struct{})}
	c, _, _ := newTestCLI(t)
	c.Stdout = out
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run([]string{"restish", "get", "--rsh-print", "bp", srv.URL + "/events"})
	}()

	select {
	case <-firstEventWritten:
	case <-time.After(time.Second):
		t.Fatal("server did not write first SSE event")
	}

	select {
	case <-out.seen:
	case err := <-errCh:
		t.Fatalf("command finished before response completed: %v", err)
	case <-time.After(time.Second):
		t.Fatal("first SSE event was not rendered before response completed")
	}

	finish()
	if err := <-errCh; err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := out.String(); !strings.Contains(got, `"n": 2`) {
		t.Fatalf("expected second event after response completed, got:\n%s", got)
	}
}

func TestSSEPlainReadableOutputFlushesBeforeResponseCompletes(t *testing.T) {
	firstEventWritten := make(chan struct{})
	finishResponse := make(chan struct{})
	var finishOnce sync.Once
	finish := func() {
		finishOnce.Do(func() { close(finishResponse) })
	}
	defer finish()

	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer does not implement http.Flusher")
		}
		fmt.Fprint(w, "data: first\n\n")
		flusher.Flush()
		close(firstEventWritten)
		<-finishResponse
		fmt.Fprint(w, "data: second\n\n")
		flusher.Flush()
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	out := &signalWriter{needle: []byte("first\n"), seen: make(chan struct{})}
	bufferedOut := bufio.NewWriter(out)
	c, _, _ := newTestCLI(t)
	c.Stdout = bufferedOut
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run([]string{"restish", "get", srv.URL + "/events", "-o", "lines"})
	}()

	select {
	case <-firstEventWritten:
	case <-time.After(time.Second):
		t.Fatal("server did not write first SSE event")
	}

	select {
	case <-out.seen:
	case err := <-errCh:
		t.Fatalf("command finished before response completed: %v", err)
	case <-time.After(time.Second):
		t.Fatal("first plain SSE event was not flushed before response completed")
	}

	finish()
	if err := <-errCh; err != nil {
		t.Fatalf("get: %v", err)
	}
	if err := bufferedOut.Flush(); err != nil {
		t.Fatalf("flush buffered output: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "second\n") {
		t.Fatalf("expected second event after response completed, got:\n%s", got)
	}
}

type signalWriter struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	needle []byte
	seen   chan struct{}
	once   sync.Once
}

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if bytes.Contains(w.buf.Bytes(), w.needle) {
		w.once.Do(func() { close(w.seen) })
	}
	return n, err
}

func (w *signalWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

var _ io.Writer = (*signalWriter)(nil)

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestStreamingMaxItemsWarns(t *testing.T) {
	for _, tt := range []struct {
		name        string
		contentType string
		body        string
		path        string
	}{
		{"sse", "text/event-stream", sseBody(`{"n":1}`, `{"n":2}`, `{"n":3}`), "/events"},
		{"ndjson", "application/x-ndjson", "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n", "/stream"},
		{"mislabeled json lines", "application/json", "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n", "/stream"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app := runStream(t, tt.contentType, tt.body, tt.path, "--rsh-max-items", "2")
			got := app.Stdout.String()
			if !strings.Contains(got, `"n": 1`) || !strings.Contains(got, `"n": 2`) {
				t.Fatalf("expected first two records, got:\n%s", got)
			}
			if strings.Contains(got, `"n": 3`) {
				t.Fatalf("unexpected third record, got:\n%s", got)
			}
			wantWarning := "streaming stopped at --rsh-max-items=2; pass 0 for unlimited"
			if !strings.Contains(app.Stderr.String(), wantWarning) {
				t.Fatalf("expected max-items warning %q, got %q", wantWarning, app.Stderr.String())
			}
		})
	}
}

func TestMislabeledJSONLinesMaxItemsDecompressesContentEncoding(t *testing.T) {
	raw := "{\"n\":1}\n{\"n\":2}\n{\"n\":3}\n"
	encoded := gzipBytes(t, raw)
	app := newTestApp(t)
	app.UseTransport(func(r *http.Request) (*http.Response, error) {
		resp := textResponse(http.StatusOK, "application/json", string(encoded), r)
		resp.Header.Set("Content-Encoding", "gzip")
		return resp, nil
	})
	app.Run("get", "https://api.example.com/stream", "--rsh-max-items", "2")

	got := app.Stdout.String()
	if !strings.Contains(got, `"n": 1`) || !strings.Contains(got, `"n": 2`) {
		t.Fatalf("expected first two decompressed lines, got:\n%s", got)
	}
	if strings.Contains(got, `"n": 3`) {
		t.Fatalf("unexpected third line, got:\n%s", got)
	}
	wantWarning := "streaming stopped at --rsh-max-items=2; pass 0 for unlimited"
	if !strings.Contains(app.Stderr.String(), wantWarning) {
		t.Fatalf("expected max-items warning %q, got %q", wantWarning, app.Stderr.String())
	}
}

func TestNDJSONDefaultHasNoItemCap(t *testing.T) {
	const records = 1002

	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		for i := 1; i <= records; i++ {
			fmt.Fprintf(w, `{"n":%d}`+"\n", i)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream", "-o", "ndjson"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := strings.TrimSpace(out.String())
	lines := strings.Split(got, "\n")
	if len(lines) != records {
		t.Fatalf("expected %d lines without a default stream cap, got %d", records, len(lines))
	}
	if strings.Contains(errOut.String(), "streaming stopped") {
		t.Fatalf("unexpected stream cap warning: %q", errOut.String())
	}
}

// TestSSEWithFilter verifies that -f applies per event and filters out
// events that don't match.
func TestSSEWithFilter(t *testing.T) {
	app := runStream(t, "text/event-stream", sseBody(`{"n":1,"type":"a"}`, `{"n":2,"type":"b"}`, `{"n":3,"type":"a"}`), "/events", "-f", ".body.data.type")
	got := app.Stdout.String()
	// Output should contain a and b (the type values).
	if !strings.Contains(got, "a") {
		t.Errorf("expected type 'a' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "b") {
		t.Errorf("expected type 'b' in output, got:\n%s", got)
	}
	// Should not contain the "n" field since we filtered to "type".
	if strings.Contains(got, `"n":`) {
		t.Errorf("unexpected 'n' field after filter, got:\n%s", got)
	}
}

func TestSSEPerEventFilterSuggestsBodyPrefix(t *testing.T) {
	app := runStream(t, "text/event-stream", sseBody(`{"n":1}`, `{"n":2}`), "/events", "-f", "data", "--rsh-max-items", "2")
	if got := app.Stderr.String(); !strings.Contains(got, "use 'body.data'") {
		t.Fatalf("expected body prefix hint, got %q", got)
	}
	if strings.Count(app.Stderr.String(), "filter returned no results") != 1 {
		t.Fatalf("expected one hint, got %q", app.Stderr.String())
	}
}

func TestSSEReadableOutputWithColor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody(`{"n":1,"type":"a"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	var out bytes.Buffer
	c.Stdout = &out
	c.Hooks().StdoutIsTerminal = func(io.Writer) bool { return true }
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	t.Setenv("COLOR", "1")
	if err := c.Run([]string{"restish", "get", srv.URL + "/events"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	stripped := stripANSI(got)
	_, body, _ := strings.Cut(stripped, "\n\n")
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), new(any)); err != nil {
		t.Fatalf("expected highlighted stream output to remain valid JSON, got %q: %v", stripped, err)
	}
	if !strings.Contains(body, "\n  ") {
		t.Errorf("expected auto output to be pretty-printed, got %q", stripped)
	}
}

func TestSSELinesOutputPlainTextStaysPlain(t *testing.T) {
	app := runStream(t, "text/event-stream", "data: plain text event\n\n", "/events", "-o", "lines")
	if got := app.Stdout.String(); got != "plain text event\n" {
		t.Fatalf("expected plain text stream output, got %q", got)
	}
}

func TestNDJSONYAMLOutputUsesFormatter(t *testing.T) {
	got := runStream(t, "application/x-ndjson", "{\"id\":1}\n{\"id\":2}\n", "/stream", "-o", "yaml").Stdout.String()
	for _, want := range []string{"id: 1", "id: 2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, `{"id":1}`) || strings.Contains(got, `"id": 1`) {
		t.Fatalf("expected stream output to use YAML formatting, got:\n%s", got)
	}
}

func TestNDJSONExplicitFormatterStreamsCompactJSON(t *testing.T) {
	got := strings.TrimSpace(runStream(t, "application/x-ndjson", "{\"id\":1}\n{\"id\":2}\n", "/stream", "-o", "ndjson").Stdout.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d:\n%s", len(lines), got)
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

func TestNDJSONOutputHandlesJSONSequenceContentType(t *testing.T) {
	got := strings.TrimSpace(runStream(t, "application/json", "{\"id\":1}\n{\"id\":2}\n", "/stream", "-o", "ndjson").Stdout.String())
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines, got %d:\n%s", len(lines), got)
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

func TestSSENDJSONOutputWithColor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody(`{"id":1}`))
		fmt.Fprint(w, sseBody(`{"id":2}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	t.Setenv("NOCOLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("COLOR", "1")
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "--rsh-max-items", "2", "-o", "ndjson", "--rsh-print", "bc"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI highlighting, got %q", got)
	}
	stripped := strings.TrimSpace(stripANSI(got))
	lines := strings.Split(stripped, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 NDJSON lines after stripping ANSI, got %d:\n%s", len(lines), stripped)
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

func TestNDJSONScannerErrorFailsCommand(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/x-ndjson"}},
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", 1024*1024+1) + "\n")),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", "--rsh-print", "b", "https://api.example.com/stream"})
	if err == nil {
		t.Fatal("expected NDJSON scanner error")
	}
	if !strings.Contains(err.Error(), "NDJSON stream error") {
		t.Fatalf("expected NDJSON stream error, got %v", err)
	}
}

func TestSSEReadErrorFailsCommand(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(errorReader{err: errors.New("broken stream")}),
			Request:    r,
		}, nil
	})

	err := c.Run([]string{"restish", "get", "--rsh-print", "b", "https://api.example.com/events"})
	if err == nil {
		t.Fatal("expected SSE read error")
	}
	if !strings.Contains(err.Error(), "SSE stream error") {
		t.Fatalf("expected SSE stream error, got %v", err)
	}
}

func TestStreamingJSONFormatterReturnsHelpfulError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"id":1}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "get", srv.URL + "/stream", "-o", "json"})
	if err == nil {
		t.Fatal("expected error for -o json on a stream, got nil")
	}
	want := "-o json for stream responses requires --rsh-collect and --rsh-max-items N. Try -o ndjson for record-by-record JSON output."
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestNDJSONCollectJSONWithMaxItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"id":1}`)
		fmt.Fprintln(w, `{"id":2}`)
		fmt.Fprintln(w, `{"id":3}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream", "--rsh-collect", "--rsh-max-items", "2", "-o", "json"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	var items []map[string]int
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatalf("output is not JSON array: %q: %v", out.String(), err)
	}
	if len(items) != 2 || items[0]["id"] != 1 || items[1]["id"] != 2 {
		t.Fatalf("items = %#v, want first two records", items)
	}
	if !strings.Contains(errOut.String(), "--rsh-max-items=2") {
		t.Fatalf("expected max-items warning, got %q", errOut.String())
	}
}

func TestSSEErrorStatusFailsBeforeStreaming(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, sseBody(`{"error":"missing"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", "--rsh-print", "b", srv.URL + "/events"})
	if exitCode(err) != 4 {
		t.Fatalf("exit code = %v, want 4 (err=%v)", exitCode(err), err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stream output before status error, got %q", out.String())
	}
}

func TestNDJSONErrorStatusCanBeIgnored(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, `{"error":"temporary"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream", "--rsh-ignore-status-code"}); err != nil {
		t.Fatalf("get with ignore-status failed: %v", err)
	}
	if !strings.Contains(out.String(), "temporary") {
		t.Fatalf("expected streamed error body, got %q", out.String())
	}
}

func exitCode(err error) int {
	var exitErr *cli.ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return 0
}

func TestSSENamedEventExposesMetadataToFilters(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: update\nid: 42\ndata: {\"n\":1}\n\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-f", ".body.event"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(out.String(), "update") {
		t.Fatalf("expected named event metadata, got %q", out.String())
	}
}

func TestSSELargeEventExceedsScannerLimit(t *testing.T) {
	largePayload := strings.Repeat("x", 2*1024*1024)
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %q\n\n", largePayload)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", "--rsh-print", "b", srv.URL + "/events"})
	if err == nil {
		t.Fatal("expected oversized SSE line error")
	}
	if !strings.Contains(err.Error(), "SSE stream line exceeds") {
		t.Fatalf("expected SSE line limit error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output for oversized line, got %q", out.String())
	}
}

func TestSSEEventDataLimit(t *testing.T) {
	chunk := strings.Repeat("x", 600*1024)
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\ndata: %s\n\n", chunk, chunk)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", "--rsh-print", "b", srv.URL + "/events"})
	if err == nil {
		t.Fatal("expected oversized SSE event error")
	}
	if !strings.Contains(err.Error(), "SSE event data exceeds") {
		t.Fatalf("expected SSE event limit error, got %v", err)
	}
}

func TestSSEMalformedLinesDoNotAbortStream(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "not-a-field\n")
		fmt.Fprint(w, "data: {\"n\":1}\n\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", "--rsh-print", "b", srv.URL + "/events"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.Contains(out.String(), `"n":1`) {
		t.Fatalf("expected event after malformed line, got %q", out.String())
	}
}

func TestSSECommentsAndRetryParsing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": ignore this comment\n")
		fmt.Fprint(w, "retry: 250\n")
		fmt.Fprint(w, "retry: nope\n")
		fmt.Fprint(w, "data: {\"n\":1}\n\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-f", ".body.retry", "-o", "lines"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "250" {
		t.Fatalf("retry output = %q, want 250", got)
	}
}

func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7E) {
				i++
			}
			i++
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
