package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestAuthCode_RefreshIgnoresWhitespaceRefreshToken(t *testing.T) {
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		return testResponse(200, "application/json", `{"access_token":"refreshed-token","token_type":"bearer","expires_in":3600,"refresh_token":"   "}`), nil
	})

	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	cacheKey := "myapi:default"
	_ = cache.Set(cacheKey, CachedToken{
		AccessToken:  "old-access",
		RefreshToken: "my-refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	})

	h := &AuthorizationCode{Cache: cache, HTTPClient: client}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{"client_id": "id1", "token_url": "https://auth.example.com/token", "_cache_key": cacheKey}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cached, err := cache.Get(cacheKey)
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}
	if cached.RefreshToken != "my-refresh-token" {
		t.Fatalf("RefreshToken = %q, want preserved token", cached.RefreshToken)
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

func TestAuthCodeRejectsInvalidDirectEndpoints(t *testing.T) {
	h := &AuthorizationCode{}
	_, _, err := h.resolveEndpoints(context.Background(), map[string]string{
		"authorize_url": "https://auth.example.com/authorize?prompt=login",
		"token_url":     "https://auth.example.com/token",
	})
	if err == nil {
		t.Fatal("expected invalid authorize_url error")
	}
	if !strings.Contains(err.Error(), "query string") {
		t.Fatalf("expected query string error, got %v", err)
	}
}

func TestAuthCode_BrowserFlow_FaviconRequestDoesNotAbort(t *testing.T) {
	var stderr bytes.Buffer
	h := &AuthorizationCode{
		Stderr: &stderr,
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
	if strings.Contains(stderr.String(), "https://auth.example.com/authorize?") {
		t.Fatalf("authorization URL should not be printed on successful browser launch without verbose: %q", stderr.String())
	}
}

func TestAuthCode_BrowserFlow_ImmediateCallbackDuringOpenBrowser(t *testing.T) {
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("code"); got != "fast-code" {
				t.Fatalf("code = %q", got)
			}
			return testResponse(200, "application/json", `{"access_token":"fast-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			callbackURL, state := mustCallbackURL(t, raw)
			resp, err := http.Get(fmt.Sprintf("%s/?state=%s&code=fast-code", callbackURL, url.QueryEscape(state)))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("callback status = %d body=%q", resp.StatusCode, string(body))
			}
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": availablePort(t),
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer fast-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer fast-token")
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

func TestAuthCode_Refresh_ClientSecretBasic(t *testing.T) {
	var got url.Values
	var authz string
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		authz = r.Header.Get("Authorization")
		if err := r.ParseForm(); err != nil {
			return testResponse(400, "text/plain", "bad form"), nil
		}
		got = r.Form
		return testResponse(200, "application/json", `{"access_token":"refreshed-token","token_type":"bearer","expires_in":3600}`), nil
	})

	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	cacheKey := "myapi:default"
	_ = cache.Set(cacheKey, CachedToken{
		AccessToken:  "old-access",
		RefreshToken: "my-refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	})

	h := &AuthorizationCode{Cache: cache, HTTPClient: client}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "https://auth.example.com/token",
		"auth_method":   "client_secret_basic",
		"_cache_key":    cacheKey,
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authz == "" {
		t.Fatal("expected Authorization header on refresh token request")
	}
	if got.Get("client_secret") != "" {
		t.Fatalf("client_secret should not be in form for basic auth: %#v", got)
	}
}

func TestAuthCode_PassesThroughAuthorizeAndTokenParams(t *testing.T) {
	var authorizeURL string
	var gotForm url.Values
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			gotForm = r.Form
			return testResponse(200, "application/json", `{"access_token":"browser-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			authorizeURL = raw
			go func() {
				callbackURL, state := mustCallbackURL(t, raw)
				resp, err := http.Get(fmt.Sprintf("%s/?state=%s&code=good-code", callbackURL, url.QueryEscape(state)))
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
	params := map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": availablePort(t),
		"audience":      "https://api.example.com/",
		"cache_key":     "local-cache-key",
		"resource":      "urn:example",
		"organization":  "acme",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsedAuthorize, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	query := parsedAuthorize.Query()
	if query.Get("audience") != "https://api.example.com/" || query.Get("resource") != "urn:example" || query.Get("organization") != "acme" {
		t.Fatalf("unexpected authorize params: %#v", query)
	}
	if query.Get("cache_key") != "" {
		t.Fatalf("cache_key should not be sent to authorize endpoint: %#v", query)
	}
	if redirectURI := query.Get("redirect_uri"); !strings.HasSuffix(redirectURI, "/") {
		t.Fatalf("redirect_uri = %q, want trailing slash", redirectURI)
	}
	if gotForm.Get("audience") != "https://api.example.com/" || gotForm.Get("resource") != "urn:example" || gotForm.Get("organization") != "acme" {
		t.Fatalf("unexpected token form params: %#v", gotForm)
	}
	if gotForm.Get("cache_key") != "" {
		t.Fatalf("cache_key should not be sent to token endpoint: %#v", gotForm)
	}
	if redirectURI := gotForm.Get("redirect_uri"); !strings.HasSuffix(redirectURI, "/") {
		t.Fatalf("token redirect_uri = %q, want trailing slash", redirectURI)
	}
}

func TestAuthCode_RedirectPath(t *testing.T) {
	var authorizeURL string
	var gotForm url.Values
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			gotForm = r.Form
			return testResponse(200, "application/json", `{"access_token":"browser-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			authorizeURL = raw
			go func() {
				callbackURL, state := mustCallbackURL(t, raw)
				resp, err := http.Get(fmt.Sprintf("%s?state=%s&code=good-code", callbackURL, url.QueryEscape(state)))
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
	err := h.OnRequest(req, map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": availablePort(t),
		"redirect_path": "/callback",
	})
	if err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	parsedAuthorize, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	if got := parsedAuthorize.Query().Get("redirect_uri"); !strings.HasSuffix(got, "/callback") {
		t.Fatalf("authorize redirect_uri = %q, want /callback", got)
	}
	if got := gotForm.Get("redirect_uri"); !strings.HasSuffix(got, "/callback") {
		t.Fatalf("token redirect_uri = %q, want /callback", got)
	}
}

func TestOAuthRedirectPathValidation(t *testing.T) {
	cases := []struct {
		name      string
		value     string
		want      string
		wantError bool
	}{
		{name: "default", want: "/"},
		{name: "callback", value: "/callback", want: "/callback"},
		{name: "relative", value: "callback", wantError: true},
		{name: "url", value: "http://localhost/callback", wantError: true},
		{name: "query", value: "/callback?x=1", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := oauthRedirectPath(tc.value)
			if tc.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("path = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAuthCode_CallbackPageReflectsTokenExchangeResult(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		bodyCh := make(chan string, 1)
		h := &AuthorizationCode{
			HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
				return testResponse(200, "application/json", `{"access_token":"browser-token","token_type":"bearer","expires_in":3600}`), nil
			}),
			OpenBrowser: func(raw string) error {
				go func() {
					callbackURL, state := mustCallbackURL(t, raw)
					resp, err := http.Get(fmt.Sprintf("%s/?state=%s&code=good-code", callbackURL, url.QueryEscape(state)))
					if err != nil {
						bodyCh <- err.Error()
						return
					}
					defer resp.Body.Close()
					body, _ := io.ReadAll(resp.Body)
					bodyCh <- string(body)
				}()
				return nil
			},
		}
		req, _ := http.NewRequest("GET", "https://api.example.com", nil)
		err := h.OnRequest(req, map[string]string{
			"client_id":     "id1",
			"authorize_url": "https://auth.example.com/authorize",
			"token_url":     "https://auth.example.com/token",
			"redirect_port": availablePort(t),
		})
		if err != nil {
			t.Fatalf("OnRequest: %v", err)
		}
		if body := <-bodyCh; !strings.Contains(body, "Authorization code received") || !strings.Contains(body, "Authentication successful") {
			t.Fatalf("callback body = %q", body)
		}
	})

	t.Run("failure", func(t *testing.T) {
		bodyCh := make(chan string, 1)
		h := &AuthorizationCode{
			HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
				return testResponse(400, "application/json", `{"error":"invalid_grant"}`), nil
			}),
			OpenBrowser: func(raw string) error {
				go func() {
					callbackURL, state := mustCallbackURL(t, raw)
					resp, err := http.Get(fmt.Sprintf("%s/?state=%s&code=bad-code", callbackURL, url.QueryEscape(state)))
					if err != nil {
						bodyCh <- err.Error()
						return
					}
					defer resp.Body.Close()
					body, _ := io.ReadAll(resp.Body)
					bodyCh <- string(body)
				}()
				return nil
			},
		}
		req, _ := http.NewRequest("GET", "https://api.example.com", nil)
		err := h.OnRequest(req, map[string]string{
			"client_id":     "id1",
			"authorize_url": "https://auth.example.com/authorize",
			"token_url":     "https://auth.example.com/token",
			"redirect_port": availablePort(t),
		})
		if err == nil {
			t.Fatal("expected token exchange error")
		}
		if body := <-bodyCh; !strings.Contains(body, "Authorization code received") || !strings.Contains(body, "Authentication failed") {
			t.Fatalf("callback body = %q", body)
		}
	})
}

func TestAuthCode_ManualCodeFallback(t *testing.T) {
	var stderr bytes.Buffer
	h := &AuthorizationCode{
		Stderr: &stderr,
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("code"); got != "manual-code" {
				t.Fatalf("code = %q", got)
			}
			return testResponse(200, "application/json", `{"access_token":"manual-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			return fmt.Errorf("browser unavailable")
		},
		Prompt: func(prompt string) (string, error) {
			if !strings.Contains(prompt, "authorization code") {
				t.Fatalf("unexpected prompt %q", prompt)
			}
			return "manual-code", nil
		},
		CanPrompt: true,
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": availablePort(t),
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer manual-token" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer manual-token")
	}
	if !strings.Contains(stderr.String(), "https://auth.example.com/authorize?") {
		t.Fatalf("expected authorization URL after browser failure, got %q", stderr.String())
	}
}

func TestAuthCode_NoBrowserManualPromptDoesNotStartCallbackListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := fmt.Sprintf("%d", ln.Addr().(*net.TCPAddr).Port)

	h := &AuthorizationCode{
		NoBrowser: true,
		CanPrompt: true,
		Prompt: func(prompt string) (string, error) {
			return "manual-code", nil
		},
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			return testResponse(200, "application/json", `{"access_token":"manual-token","token_type":"bearer","expires_in":3600}`), nil
		}),
	}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err = h.OnRequest(req, map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": port,
	})
	if err != nil {
		t.Fatalf("manual prompt should not bind occupied callback port: %v", err)
	}
}

func TestAuthCode_RefreshNetworkFailureDoesNotFallback(t *testing.T) {
	h := &AuthorizationCode{
		Cache: NewTokenCache(filepath.Join(t.TempDir(), "tokens.json")),
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		}),
		OpenBrowser: func(raw string) error {
			t.Fatal("browser flow should not run after non-invalid_grant refresh failure")
			return nil
		},
	}

	cacheKey := "myapi:default"
	if err := h.Cache.Set(cacheKey, CachedToken{
		AccessToken:  "old-access",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{
		"client_id":  "id1",
		"token_url":  "https://auth.example.com/token",
		"_cache_key": cacheKey,
	})
	if err == nil {
		t.Fatal("expected refresh failure")
	}
	if !strings.Contains(err.Error(), "dial failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultOpenBrowserReturnsAfterStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command not portable to Windows")
	}
	oldCommand := openBrowserCommand
	openBrowserCommand = func(rawURL string) *exec.Cmd {
		return exec.Command("sleep", "2")
	}
	t.Cleanup(func() { openBrowserCommand = oldCommand })

	start := time.Now()
	if err := DefaultOpenBrowser("https://example.com"); err != nil {
		t.Fatalf("DefaultOpenBrowser: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("DefaultOpenBrowser waited for child process: %v", elapsed)
	}
}

func TestDefaultOpenBrowserCommandUsesArgumentSeparator(t *testing.T) {
	cmd := defaultOpenBrowserCommand("-https://example.com")
	args := strings.Join(cmd.Args, "\x00")
	if !strings.Contains(args, "\x00--\x00-https://example.com") {
		t.Fatalf("browser command should pass -- before URL, got %#v", cmd.Args)
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
