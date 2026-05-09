package cli_test

import (
	"bufio"
	"bytes"
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

// TestSSEThreeEvents verifies that three SSE events are each printed to stdout.
func TestSSEThreeEvents(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody(`{"n":1}`, `{"n":2}`, `{"n":3}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	for _, want := range []string{`"n":1`, `"n":2`, `"n":3`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	// Each event should be on its own line.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 output lines, got %d:\n%s", len(lines), got)
	}
}

func TestSSEDataFieldWithoutColonPreservesBlankLine(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\ndata\ndata: third\n\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-f", "body.data", "-o", "lines"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	if got, want := out.String(), "first\n\nthird\n"; got != want {
		t.Fatalf("SSE data output = %q, want %q", got, want)
	}
}

// TestNDJSONThreeLines verifies that three NDJSON lines are each printed to stdout.
func TestNDJSONThreeLines(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"n":1}`)
		fmt.Fprintln(w, `{"n":2}`)
		fmt.Fprintln(w, `{"n":3}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	for _, want := range []string{`"n":1`, `"n":2`, `"n":3`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, got)
		}
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 output lines, got %d:\n%s", len(lines), got)
	}
}

func TestNDJSONLineLimitUsesMaxBodySize(t *testing.T) {
	message := strings.Repeat("x", 1200*1024)
	line := `{"message":"` + message + `"}`
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, line)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream", "--rsh-max-body-size", "2", "-f", "body.message", "-o", "lines"}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != message {
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

	out := &signalWriter{needle: []byte(`"n":1`), seen: make(chan struct{})}
	c, _, _ := newTestCLI(t)
	c.Stdout = out
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run([]string{"restish", "get", srv.URL + "/stream"})
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
	if got := out.String(); !strings.Contains(got, `"n":2`) {
		t.Fatalf("expected second line after response completed, got:\n%s", got)
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

	out := &signalWriter{needle: []byte(`"n":1`), seen: make(chan struct{})}
	c, _, _ := newTestCLI(t)
	c.Stdout = out
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run([]string{"restish", "get", srv.URL + "/events"})
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
	if got := out.String(); !strings.Contains(got, `"n":2`) {
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
		errCh <- c.Run([]string{"restish", "get", srv.URL + "/events", "-o", "readable"})
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

// TestSSEMaxItems verifies that --rsh-max-items 2 stops after 2 SSE events.
func TestSSEMaxItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody(`{"n":1}`, `{"n":2}`, `{"n":3}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "--rsh-max-items", "2"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	// Events 1 and 2 should be present; event 3 should not.
	if !strings.Contains(got, `"n":1`) {
		t.Errorf("expected event 1 in output, got:\n%s", got)
	}
	if !strings.Contains(got, `"n":2`) {
		t.Errorf("expected event 2 in output, got:\n%s", got)
	}
	if strings.Contains(got, `"n":3`) {
		t.Errorf("unexpected event 3 in output (should be stopped by --rsh-max-items 2), got:\n%s", got)
	}
	wantWarning := "streaming stopped at --rsh-max-items=2; pass 0 for unlimited"
	if !strings.Contains(errOut.String(), wantWarning) {
		t.Fatalf("expected max-items warning %q, got %q", wantWarning, errOut.String())
	}
}

func TestNDJSONMaxItemsWarns(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"n":1}`)
		fmt.Fprintln(w, `{"n":2}`)
		fmt.Fprintln(w, `{"n":3}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream", "--rsh-max-items", "2"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `"n":1`) || !strings.Contains(got, `"n":2`) {
		t.Fatalf("expected first two lines, got:\n%s", got)
	}
	if strings.Contains(got, `"n":3`) {
		t.Fatalf("unexpected third line, got:\n%s", got)
	}
	wantWarning := "streaming stopped at --rsh-max-items=2; pass 0 for unlimited"
	if !strings.Contains(errOut.String(), wantWarning) {
		t.Fatalf("expected max-items warning %q, got %q", wantWarning, errOut.String())
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
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Events: n=1 (type=a), n=2 (type=b), n=3 (type=a)
		fmt.Fprint(w, sseBody(`{"n":1,"type":"a"}`, `{"n":2,"type":"b"}`, `{"n":3,"type":"a"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	// Select only the "type" field from each structured SSE event.
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-f", ".body.data.type"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
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
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	t.Setenv("COLOR", "1")
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-o", "readable"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	stripped := stripANSI(got)
	if err := json.Unmarshal([]byte(strings.TrimSpace(stripped)), new(any)); err != nil {
		t.Fatalf("expected highlighted stream output to remain valid JSON, got %q: %v", stripped, err)
	}
	if !strings.Contains(stripped, "\n  ") {
		t.Errorf("expected readable output to be pretty-printed, got %q", stripped)
	}
}

func TestSSEReadableOutputPlainTextStaysPlain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: plain text event\n\n")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-o", "readable"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	if got != "plain text event\n" {
		t.Fatalf("expected plain text stream output, got %q", got)
	}
}

func TestNDJSONYAMLOutputUsesFormatter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"id":1}`)
		fmt.Fprintln(w, `{"id":2}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream", "-o", "yaml"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
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
	mux := http.NewServeMux()
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"id":1}`)
		fmt.Fprintln(w, `{"id":2}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/stream", "-o", "ndjson"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := strings.TrimSpace(out.String())
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
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "--rsh-max-items", "2", "-o", "ndjson"}); err != nil {
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

	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", "https://api.example.com/stream"})
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

	err := c.Run([]string{"restish", "get", "https://api.example.com/events"})
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
	want := "-o json cannot be used with an unbounded stream response. Try -o ndjson for record-by-record JSON output."
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
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
	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", srv.URL + "/events"})
	if exitCode(err) != 1 {
		t.Fatalf("exit code = %v, want 1 (err=%v)", exitCode(err), err)
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
	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", srv.URL + "/events"})
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
	err := c.Run([]string{"restish", "--rsh-max-body-size", "1", "get", srv.URL + "/events"})
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
	if err := c.Run([]string{"restish", "get", srv.URL + "/events"}); err != nil {
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
