package request

import (
	"net/http"
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
