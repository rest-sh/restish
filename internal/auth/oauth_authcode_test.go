package auth

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestAuthCode_RefreshToken verifies that an expired cached token with a
// refresh token causes the handler to exchange the refresh token for a new
// access token without opening a browser.
func TestAuthCode_RefreshToken(t *testing.T) {
	var refreshCallCount atomic.Int32
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return testResponse(400, "text/plain", "bad form"), nil
		}
		if gt := r.FormValue("grant_type"); gt != "refresh_token" {
			t.Fatalf("unexpected grant_type %q", gt)
		}
		refreshCallCount.Add(1)
		return testResponse(200, "application/json", `{"access_token":"refreshed-token","token_type":"bearer","expires_in":3600}`), nil
	})

	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	cacheKey := "myapi:default"

	// Pre-populate cache with expired token + refresh token.
	_ = cache.Set(cacheKey, CachedToken{
		AccessToken:  "old-access",
		RefreshToken: "my-refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	})

	// OpenBrowser must NOT be called (refresh avoids the browser flow).
	h := &AuthorizationCode{
		Cache:      cache,
		HTTPClient: client,
		OpenBrowser: func(url string) error {
			t.Errorf("OpenBrowser called unexpectedly with %q", url)
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":  "id1",
		"token_url":  "https://auth.example.com/token",
		"_cache_key": cacheKey,
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer refreshed-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer refreshed-token")
	}
	if n := refreshCallCount.Load(); n != 1 {
		t.Errorf("expected refresh endpoint called once, got %d", n)
	}
}

func TestAuthCode_RefreshPreservesExistingRefreshToken(t *testing.T) {
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return testResponse(400, "text/plain", "bad form"), nil
		}
		return testResponse(200, "application/json", `{"access_token":"refreshed-token","token_type":"bearer","expires_in":3600}`), nil
	})

	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	cacheKey := "myapi:default"
	_ = cache.Set(cacheKey, CachedToken{
		AccessToken:  "old-access",
		RefreshToken: "my-refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	})

	h := &AuthorizationCode{
		Cache:      cache,
		HTTPClient: client,
		OpenBrowser: func(url string) error {
			t.Fatalf("OpenBrowser called unexpectedly with %q", url)
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":  "id1",
		"token_url":  "https://auth.example.com/token",
		"_cache_key": cacheKey,
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cached, err := cache.Get(cacheKey)
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}
	if cached == nil {
		t.Fatal("expected cached token")
	}
	if cached.RefreshToken != "my-refresh-token" {
		t.Fatalf("RefreshToken = %q, want %q", cached.RefreshToken, "my-refresh-token")
	}
}

// TestAuthCode_ValidCachedToken verifies that a still-valid cached token is
// used without contacting any endpoint.
func TestAuthCode_ValidCachedToken(t *testing.T) {
	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	cacheKey := "myapi:default"

	_ = cache.Set(cacheKey, CachedToken{
		AccessToken: "valid-token",
		Expiry:      time.Now().Add(time.Hour),
	})

	h := &AuthorizationCode{
		Cache: cache,
		OpenBrowser: func(url string) error {
			t.Error("OpenBrowser should not be called for a valid cached token")
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":  "id1",
		"token_url":  "http://unused",
		"_cache_key": cacheKey,
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer valid-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer valid-token")
	}
}

func TestAuthCode_BrowserFlow_FaviconRequestDoesNotAbort(t *testing.T) {
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://auth.example.com/token" {
				t.Fatalf("unexpected URL %q", r.URL.String())
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("code"); got != "good-code" {
				t.Fatalf("code = %q", got)
			}
			return testResponse(200, "application/json", `{"access_token":"browser-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			go func() {
				callbackURL, state := mustCallbackURL(t, raw)
				resp1, err := http.Get(callbackURL + "/favicon.ico")
				if err != nil {
					t.Errorf("favicon request failed: %v", err)
					return
				}
				defer resp1.Body.Close()
				if resp1.StatusCode != http.StatusNotFound {
					t.Errorf("favicon status = %d, want %d", resp1.StatusCode, http.StatusNotFound)
				}
				resp2, err := http.Get(fmt.Sprintf("%s/?state=%s&code=good-code", callbackURL, url.QueryEscape(state)))
				if err != nil {
					t.Errorf("callback request failed: %v", err)
					return
				}
				defer resp2.Body.Close()
				if resp2.StatusCode != http.StatusOK {
					body, _ := io.ReadAll(resp2.Body)
					t.Errorf("callback status = %d body=%q", resp2.StatusCode, string(body))
				}
			}()
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	redirectPort := availablePort(t)
	params := map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": redirectPort,
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer browser-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer browser-token")
	}
}

func TestAuthCode_BrowserFlow_TwoStrayPreflightsDoNotDeadlock(t *testing.T) {
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("code"); got != "final-code" {
				t.Fatalf("code = %q", got)
			}
			return testResponse(200, "application/json", `{"access_token":"browser-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			go func() {
				callbackURL, state := mustCallbackURL(t, raw)
				for _, path := range []string{"/favicon.ico", "/robots.txt"} {
					resp, err := http.Get(callbackURL + path)
					if err != nil {
						t.Errorf("preflight %s failed: %v", path, err)
						return
					}
					resp.Body.Close()
				}
				resp, err := http.Get(fmt.Sprintf("%s/callback?state=%s&code=final-code", callbackURL, url.QueryEscape(state)))
				if err != nil {
					t.Errorf("callback request failed: %v", err)
					return
				}
				resp.Body.Close()
			}()
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	redirectPort := availablePort(t)
	params := map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": redirectPort,
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer browser-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer browser-token")
	}
}

func mustCallbackURL(t *testing.T, rawAuthorizeURL string) (string, string) {
	t.Helper()
	u, err := url.Parse(rawAuthorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	q := u.Query()
	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" {
		t.Fatal("missing redirect_uri")
	}
	state := q.Get("state")
	if state == "" {
		t.Fatal("missing state")
	}
	redirectURI = strings.TrimSuffix(redirectURI, "/")
	return redirectURI, state
}

func availablePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate port: %v", err)
	}
	defer ln.Close()
	return fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)
}
