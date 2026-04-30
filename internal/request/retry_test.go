package request

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWaitDurationUsesEqualJitterBounds(t *testing.T) {
	rt := retryTransport{baseDelay: time.Second}
	for attempt := 1; attempt <= 5; attempt++ {
		base := time.Second * (1 << uint(attempt-1))
		if base > 30*time.Second {
			base = 30 * time.Second
		}
		wait := rt.waitDuration(nil, attempt)
		if wait < base/2 {
			t.Fatalf("attempt %d: wait %v below lower bound %v", attempt, wait, base/2)
		}
		if wait >= base+base/2 {
			t.Fatalf("attempt %d: wait %v above upper bound %v", attempt, wait, base+base/2)
		}
	}
}

func TestWaitDurationHonorsRetryAfterBeforeJitter(t *testing.T) {
	rt := retryTransport{baseDelay: time.Second}
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"5"}}}
	if wait := rt.waitDuration(resp, 3); wait != 5*time.Second {
		t.Fatalf("waitDuration = %v, want %v", wait, 5*time.Second)
	}
}

func TestWaitDurationHonorsRetryAfterZero(t *testing.T) {
	rt := retryTransport{baseDelay: time.Second}
	resp := &http.Response{Header: http.Header{"Retry-After": []string{"0"}}}
	if wait := rt.waitDuration(resp, 3); wait != 0 {
		t.Fatalf("waitDuration = %v, want 0", wait)
	}
}

func TestWaitDurationHonorsXRetryIn(t *testing.T) {
	rt := retryTransport{baseDelay: time.Second}
	resp := &http.Response{Header: http.Header{"X-Retry-In": []string{"3"}}}
	if wait := rt.waitDuration(resp, 2); wait != 3*time.Second {
		t.Fatalf("waitDuration = %v, want %v", wait, 3*time.Second)
	}
}

func TestWaitDurationPrefersRetryAfterOverXRetryIn(t *testing.T) {
	rt := retryTransport{baseDelay: time.Second}
	resp := &http.Response{Header: http.Header{
		"Retry-After": []string{"5"},
		"X-Retry-In":  []string{"3"},
	}}
	if wait := rt.waitDuration(resp, 2); wait != 5*time.Second {
		t.Fatalf("waitDuration = %v, want Retry-After value", wait)
	}
}

func TestRetryTransportRetries429AndLogsProgress(t *testing.T) {
	var log bytes.Buffer
	attempts := 0
	rt := retryTransport{
		baseDelay: time.Millisecond,
		maxRetry:  1,
		logger:    &log,
		inner: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"0"}},
					Body:       io.NopCloser(strings.NewReader("retry")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}),
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.com/items", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer resp.Body.Close()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !strings.Contains(log.String(), "warning: retry 1/1") {
		t.Fatalf("expected retry log, got %q", log.String())
	}
}

func TestRetryTransportSkipsUnsafeMethodByDefault(t *testing.T) {
	attempts := 0
	rt := retryTransport{
		baseDelay: time.Millisecond,
		maxRetry:  2,
		inner: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Retry-After": []string{"0"}},
				Body:       io.NopCloser(strings.NewReader("retry")),
			}, nil
		}),
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.example.com/items", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer resp.Body.Close()
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestRetryTransportRetriesUnsafeMethodWhenOptedIn(t *testing.T) {
	attempts := 0
	rt := retryTransport{
		baseDelay:   time.Millisecond,
		maxRetry:    1,
		retryUnsafe: true,
		inner: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Header:     http.Header{"Retry-After": []string{"0"}},
					Body:       io.NopCloser(strings.NewReader("retry")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("ok")),
			}, nil
		}),
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://api.example.com/items", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("body")), nil
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	defer resp.Body.Close()
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestRetryTransportReturnsLatestErrorWithoutStaleResponse(t *testing.T) {
	rt := retryTransport{
		baseDelay: time.Millisecond,
		maxRetry:  1,
		inner: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("X-Attempt") == "" {
				req.Header.Set("X-Attempt", "1")
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader("bad gateway")),
				}, nil
			}
			return nil, errors.New("dial failed")
		}),
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.example.com/items", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := rt.RoundTrip(req)
	if err == nil || err.Error() != "dial failed" {
		t.Fatalf("err = %v, want dial failed", err)
	}
	if resp != nil {
		t.Fatalf("resp = %#v, want nil", resp)
	}
}
