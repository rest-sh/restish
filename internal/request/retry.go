package request

import (
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

// retryTransport wraps an inner RoundTripper and retries on network errors
// and 5xx responses with exponential backoff + jitter.  4xx responses are
// returned immediately without retrying.
type retryTransport struct {
	inner     http.RoundTripper
	maxRetry  int
	baseDelay time.Duration
}

func (rt retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for attempt := 0; attempt <= rt.maxRetry; attempt++ {
		if attempt > 0 {
			// Can only retry if we can recreate the body.
			if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
				// Body is not replayable; return whatever we have.
				return resp, err
			}

			wait := rt.waitDuration(resp, attempt)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(wait):
			}

			// Refresh the request body for the retry.
			if req.GetBody != nil {
				newBody, gbErr := req.GetBody()
				if gbErr != nil {
					return nil, gbErr
				}
				req = req.Clone(req.Context())
				req.Body = newBody
			}
		}

		resp, err = rt.inner.RoundTrip(req)

		if err != nil {
			// Network-level error — retry.
			continue
		}

		if resp.StatusCode >= 500 {
			if attempt < rt.maxRetry {
				// Not the final attempt — drain and close so the connection
				// can be reused, then retry.
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				continue
			}
			// Final attempt — leave body open for the caller.
			return resp, nil
		}

		// Success or 4xx — return as-is.
		return resp, nil
	}

	return resp, err
}

// waitDuration returns the duration to wait before the next attempt.
// It honours the Retry-After response header when present; otherwise it
// computes exponential backoff with ±25 % jitter.
func (rt retryTransport) waitDuration(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			// Integer seconds form.
			if secs, parseErr := strconv.Atoi(ra); parseErr == nil && secs > 0 {
				return time.Duration(secs) * time.Second
			}
			// HTTP-date form.
			if t, parseErr := http.ParseTime(ra); parseErr == nil {
				if wait := time.Until(t); wait > 0 {
					return wait
				}
			}
		}
	}

	// Exponential backoff: baseDelay * 2^(attempt-1), capped at 30 s.
	base := rt.baseDelay * (1 << uint(attempt-1))
	if base > 30*time.Second {
		base = 30 * time.Second
	}
	// Add ±25 % jitter.
	jitter := time.Duration(rand.Int64N(int64(base)/2)) - base/4
	wait := base + jitter
	if wait < 0 {
		wait = 0
	}
	return wait
}
