package cli_test

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

// TestRetrySucceedsAfterTransientFailures verifies that when the server
// returns 503 twice and then 200, the request ultimately succeeds.
func TestRetrySucceedsAfterTransientFailures(t *testing.T) {
	var callCount atomic.Int32
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		n := callCount.Add(1)
		if n <= 2 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
		return jsonResponse(http.StatusOK, `{"ok":true}`), nil
	})

	if err := c.Run([]string{"restish", "get", "--rsh-no-cache", "https://api.example.com/items"}); err != nil {
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
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		callCount.Add(1)
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})

	// Ignore the exit-code error; we just want to count server hits.
	_ = c.Run([]string{"restish", "get", "--rsh-no-cache", "--rsh-ignore-status-code", "https://api.example.com/items"})
	if n := callCount.Load(); n != 1 {
		t.Errorf("4xx should not be retried; expected 1 call, got %d", n)
	}
}

// TestRetryZeroDisablesRetries verifies that --rsh-retry 0 sends exactly one
// request even when the server always returns 503.
func TestRetryZeroDisablesRetries(t *testing.T) {
	var callCount atomic.Int32
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		callCount.Add(1)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})

	_ = c.Run([]string{"restish", "get", "--rsh-no-cache", "--rsh-ignore-status-code", "--rsh-retry", "0", "https://api.example.com/items"})
	if n := callCount.Load(); n != 1 {
		t.Errorf("--rsh-retry 0: expected 1 call, got %d", n)
	}
}

// TestRetryAfterHeaderRespected verifies that the Retry-After header value is
// used as the wait duration (the test uses a 0-second value to stay fast).
func TestRetryAfterHeaderRespected(t *testing.T) {
	var callCount atomic.Int32
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		n := callCount.Add(1)
		if n == 1 {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Retry-After": []string{"0"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
		return jsonResponse(http.StatusOK, `{"ok":true}`), nil
	})

	if err := c.Run([]string{"restish", "get", "--rsh-no-cache", "https://api.example.com/items"}); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if n := callCount.Load(); n != 2 {
		t.Errorf("expected 2 server calls, got %d", n)
	}
}
