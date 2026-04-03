package cli_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestRetrySucceedsAfterTransientFailures verifies that when the server
// returns 503 twice and then 200, the request ultimately succeeds.
func TestRetrySucceedsAfterTransientFailures(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", "--rsh-no-cache", srv.URL}); err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if n := callCount.Load(); n != 3 {
		t.Errorf("expected 3 server calls (2 failures + 1 success), got %d", n)
	}
}

// TestRetryNotAttemptedFor4xx verifies that 4xx responses are returned
// immediately without any retry.
func TestRetryNotAttemptedFor4xx(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI()
	// Ignore the exit-code error; we just want to count server hits.
	_ = c.Run([]string{"restish", "get", "--rsh-no-cache", "--rsh-ignore-status-code", srv.URL})
	if n := callCount.Load(); n != 1 {
		t.Errorf("4xx should not be retried; expected 1 call, got %d", n)
	}
}

// TestRetryZeroDisablesRetries verifies that --rsh-retry 0 sends exactly one
// request even when the server always returns 503.
func TestRetryZeroDisablesRetries(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI()
	_ = c.Run([]string{"restish", "get", "--rsh-no-cache", "--rsh-ignore-status-code", "--rsh-retry", "0", srv.URL})
	if n := callCount.Load(); n != 1 {
		t.Errorf("--rsh-retry 0: expected 1 call, got %d", n)
	}
}

// TestRetryAfterHeaderRespected verifies that the Retry-After header value is
// used as the wait duration (the test uses a 0-second value to stay fast).
func TestRetryAfterHeaderRespected(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "get", "--rsh-no-cache", srv.URL}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if n := callCount.Load(); n != 2 {
		t.Errorf("expected 2 server calls, got %d", n)
	}
}
