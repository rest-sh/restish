package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/spf13/cobra"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestCommandContextCancelsPaginationRequests(t *testing.T) {
	c := New()
	c.Stdout = &bytes.Buffer{}
	c.Stderr = &bytes.Buffer{}

	ctx, cancel := context.WithCancel(context.Background())
	reqCount := 0
	c.HTTPTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		reqCount++
		switch reqCount {
		case 1:
			cancel()
			return &http.Response{
				StatusCode: 200,
				Status:     "200 OK",
				Proto:      "HTTP/1.1",
				Header: http.Header{
					"Content-Type": []string{"application/json"},
					"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
				},
				Body:    io.NopCloser(strings.NewReader(`[1]`)),
				Request: r,
			}, nil
		default:
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
	})

	root := c.newRootCmd()
	root.SetContext(ctx)
	root.SetArgs([]string{"get", "https://api.example.com/items"})

	err := root.Execute()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if reqCount != 1 {
		t.Fatalf("expected pagination to stop before a second request when context is canceled, got %d requests", reqCount)
	}
}

func TestCommandContextCancelsAPISyncDiscovery(t *testing.T) {
	c := New()
	c.Stdout = &bytes.Buffer{}
	c.Stderr = &bytes.Buffer{}
	c.SpecCachePath = t.TempDir()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"example": {BaseURL: "https://api.example.com"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.HTTPTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	err := c.runAPISync(cmd, []string{"example"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
