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
				// Only drain and retry if we can recreate the body (or there is none).
				// If GetBody is nil the body is not replayable; return now with body intact.
				if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
					return resp, nil
				}
				// Drain and close so the connection can be reused, then retry.
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

func (rt retryTransport) Close() error {
	if closer, ok := rt.inner.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

func (rt retryTransport) CloseIdleConnections() {
	if closer, ok := rt.inner.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}

// waitDuration returns the duration to wait before the next attempt.
// It honours the Retry-After response header when present (capped at 60s);
// otherwise it computes exponential backoff with ±25 % jitter.
func (rt retryTransport) waitDuration(resp *http.Response, attempt int) time.Duration {
	if resp != nil {
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			// Integer seconds form.
			if secs, parseErr := strconv.Atoi(ra); parseErr == nil && secs > 0 {
				wait := time.Duration(secs) * time.Second
				if wait > 60*time.Second {
					wait = 60 * time.Second
				}
				return wait
			}
			// HTTP-date form.
			if t, parseErr := http.ParseTime(ra); parseErr == nil {
				if wait := time.Until(t); wait > 0 {
					if wait > 60*time.Second {
						wait = 60 * time.Second
					}
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
	// Equal jitter: base/2 + random[0, base). Guard against base < 2 to avoid
	// rand.Int64N(0) panic.
	if int64(base) < 2 {
		return base
	}
	return base/2 + time.Duration(rand.Int64N(int64(base)))
}
