package cli_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchResponseSendsNegotiationHeaders(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return &http.Response{
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	if _, err := c.FetchResponse(context.Background(), http.MethodGet, "https://api.example.com/items", "", nil); err != nil {
		t.Fatalf("FetchResponse: %v", err)
	}

	req := rr.Last()
	if req == nil {
		t.Fatal("expected request to be captured")
	}
	if got := req.Header.Get("Accept"); got == "" {
		t.Fatal("FetchResponse did not send Accept header")
	}
	if got := req.Header.Get("Accept-Encoding"); got == "" {
		t.Fatal("FetchResponse did not send Accept-Encoding header")
	}
	if got := req.Header.Get("User-Agent"); !strings.HasPrefix(got, "restish/") {
		t.Fatalf("FetchResponse User-Agent = %q, want restish/<version>", got)
	}
}
