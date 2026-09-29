package cli_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/auth"
	authpkg "github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/cli"
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

func TestFetchResponseBrowserPolicy(test *testing.T) {
	for _, noBrowser := range []bool{false, true} {
		test.Run(fmt.Sprintf("no-browser=%t", noBrowser), func(test *testing.T) {
			client, output, diagnostics := newTestCLI(test)
			port, err := authpkg.FreePort()
			if err != nil {
				test.Fatal(err)
			}
			test.Setenv("BROWSER", filepath.Join(test.TempDir(), "missing-browser"))
			client.Hooks().ConfigPath = writeAPIConfig(test, fmt.Sprintf(`{
  "apis": {"myapi": {"base_url": "https://api.example.com", "profiles": {
    "manual": {"headers": ["X-Profile: manual"], "auth": {
      "type": "oauth-authorization-code", "params": {
        "client_id": "public-client", "redirect_port": %q,
        "authorize_url": "https://oauth.example.com/authorize",
        "token_url": "https://oauth.example.com/token"
      }
    }}
  }}}
}`, port))
			client.Hooks().PassReader = strings.NewReader("manual-code\n")
			var recorder requestRecorder
			exchanges := 0
			useTransport(client, func(request *http.Request) (*http.Response, error) {
				switch request.URL.String() {
				case "https://oauth.example.com/token":
					exchanges++
					if err := request.ParseForm(); err != nil {
						test.Fatal(err)
					}
					if request.FormValue("code") != "manual-code" {
						test.Fatalf("unexpected authorization code: %q", request.FormValue("code"))
					}
					return oauthTokenResponse(request, "manual-token"), nil
				case "https://api.example.com/items":
					recorder.capture(request)
					return jsonResponse(http.StatusOK, `{"ok":true}`), nil
				default:
					return jsonResponse(http.StatusNotFound, `{}`), nil
				}
			})
			if err := client.Run([]string{"restish", "--help"}); err != nil {
				test.Fatal(err)
			}
			output.Reset()
			headers := []string{"X-Request: explicit"}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if noBrowser {
				_, err = client.FetchResponseWithOptions(ctx, http.MethodGet, "https://api.example.com/items", cli.FetchOptions{ProfileName: "manual", Headers: headers, NoBrowser: true})
			} else {
				_, err = client.FetchResponse(ctx, http.MethodGet, "https://api.example.com/items", "manual", headers)
			}
			if err != nil {
				test.Fatal(err)
			}
			request := recorder.Last()
			if request == nil || request.Header.Get("Authorization") != "Bearer manual-token" || request.Header.Get("X-Profile") != "manual" || request.Header.Get("X-Request") != "explicit" {
				test.Fatalf("profile authentication or headers not applied: %v", request)
			}
			attemptedBrowser := strings.Contains(diagnostics.String(), "Could not open browser")
			if attemptedBrowser == noBrowser {
				test.Fatalf("browser policy violated: noBrowser=%t diagnostics=%q", noBrowser, diagnostics.String())
			}
			if !strings.Contains(diagnostics.String(), "Paste the authorization code") || output.Len() != 0 {
				test.Fatalf("missing manual prompt or unexpected response output: %q %q", output.String(), diagnostics.String())
			}
			if noBrowser {
				diagnostics.Reset()
				before := exchanges
				if _, err := client.FetchResponse(ctx, http.MethodGet, "https://api.example.com/items", "manual", headers); err != nil {
					test.Fatal(err)
				}
				if exchanges != before || strings.Contains(diagnostics.String(), "Paste the authorization code") || strings.Contains(diagnostics.String(), "Could not open browser") {
					test.Fatalf("browser policy changed credential-cache identity: %q", diagnostics.String())
				}
				if err := auth.SaveTokenCache(client.Hooks().TokenCachePath, map[string]auth.CachedToken{}); err != nil {
					test.Fatal(err)
				}
				client.Hooks().PassReader = strings.NewReader("manual-code\n")
				if _, err := client.FetchResponse(ctx, http.MethodGet, "https://api.example.com/items", "manual", headers); err != nil {
					test.Fatal(err)
				}
				if !strings.Contains(diagnostics.String(), "Could not open browser") || !strings.Contains(diagnostics.String(), "Paste the authorization code") {
					test.Fatalf("per-call browser policy leaked into legacy fetch: %q", diagnostics.String())
				}
			}
		})
	}
}

func TestFetchResponseWithOptionsPreservesCallerContext(test *testing.T) {
	client, _, _ := newTestCLI(test)
	ctx, cancel := context.WithCancel(context.Background())
	useTransport(client, func(request *http.Request) (*http.Response, error) {
		cancel()
		select {
		case <-request.Context().Done():
			return nil, request.Context().Err()
		case <-time.After(time.Second):
			test.Fatal("fetch detached the caller's cancellation")
			return nil, context.DeadlineExceeded
		}
	})
	defer cancel()
	_, err := client.FetchResponseWithOptions(ctx, http.MethodGet, "https://api.example.com/items", cli.FetchOptions{NoBrowser: true})
	if !errors.Is(err, context.Canceled) {
		test.Fatalf("expected caller cancellation, got %v", err)
	}
}
