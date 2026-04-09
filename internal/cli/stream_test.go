package cli_test

import (
	"bytes"
	"fmt"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
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

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
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

// TestSSEMaxEvents verifies that --rsh-max-events 2 stops after 2 SSE events.
func TestSSEMaxEvents(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody(`{"n":1}`, `{"n":2}`, `{"n":3}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "--rsh-max-events", "2"}); err != nil {
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
		t.Errorf("unexpected event 3 in output (should be stopped by --rsh-max-events 2), got:\n%s", got)
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

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	// Select only the "type" field from each event.
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-f", ".body.type"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	// Output should contain "a" and "b" (the type values).
	if !strings.Contains(got, `"a"`) {
		t.Errorf("expected type 'a' in output, got:\n%s", got)
	}
	if !strings.Contains(got, `"b"`) {
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

	c, _, _ := newTestCLI()
	var out bytes.Buffer
	c.Stdout = &out
	c.ConfigPath = t.TempDir() + "/restish.json"
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

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL + "/events", "-o", "readable"}); err != nil {
		t.Fatalf("get: %v", err)
	}

	got := out.String()
	if got != "plain text event\n" {
		t.Fatalf("expected plain text stream output, got %q", got)
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
