package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
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

func TestAuthCode_ConcurrentRefreshReusesStoredToken(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "tokens.cbor")
	cache := NewTokenCache(cacheFile)
	cacheKey := "myapi:default"
	if err := cache.Set(cacheKey, CachedToken{
		AccessToken:  "old-access",
		RefreshToken: "my-refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	var refreshCalls atomic.Int32
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			return testResponse(400, "text/plain", "bad form"), nil
		}
		if gt := r.FormValue("grant_type"); gt != "refresh_token" {
			t.Fatalf("unexpected grant_type %q", gt)
		}
		if got := r.FormValue("refresh_token"); got != "my-refresh-token" {
			t.Fatalf("refresh_token = %q", got)
		}
		if refreshCalls.Add(1) == 1 {
			close(started)
			<-release
		}
		return testResponse(200, "application/json", `{"access_token":"refreshed-token","token_type":"bearer","expires_in":3600,"refresh_token":"rotated-refresh"}`), nil
	})

	params := map[string]string{
		"client_id":  "id1",
		"token_url":  "https://auth.example.com/token",
		"_cache_key": cacheKey,
	}
	h1 := &AuthorizationCode{Cache: NewTokenCache(cacheFile), HTTPClient: client}
	h2 := &AuthorizationCode{Cache: NewTokenCache(cacheFile), HTTPClient: client}
	run := func(h *AuthorizationCode) (string, error) {
		req, _ := http.NewRequest("GET", "https://api.example.com", nil)
		err := h.OnRequest(req, params)
		return req.Header.Get("Authorization"), err
	}

	type result struct {
		auth string
		err  error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		auth, err := run(h1)
		results <- result{auth: auth, err: err}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first refresh did not start")
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		auth, err := run(h2)
		results <- result{auth: auth, err: err}
	}()
	close(release)
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			t.Fatalf("OnRequest: %v", result.err)
		}
		if result.auth != "Bearer refreshed-token" {
			t.Fatalf("Authorization = %q, want refreshed token", result.auth)
		}
	}
	if got := refreshCalls.Load(); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
	cached, err := NewTokenCache(cacheFile).Get(cacheKey)
	if err != nil {
		t.Fatalf("cache.Get: %v", err)
	}
	if cached == nil || cached.RefreshToken != "rotated-refresh" {
		t.Fatalf("cached token = %+v, want rotated refresh token", cached)
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

func TestAuthCodeResolvesRelativeEndpoints(t *testing.T) {
	h := &AuthorizationCode{}
	authorizeURL, tokenURL, err := h.resolveEndpoints(context.Background(), map[string]string{
		"authorize_url": "oauth2/authorize",
		"token_url":     "/oauth2/token",
		"_base_url":     "https://api.example.com/v1",
	})
	if err != nil {
		t.Fatalf("resolveEndpoints: %v", err)
	}
	if authorizeURL != "https://api.example.com/v1/oauth2/authorize" {
		t.Fatalf("authorizeURL = %q", authorizeURL)
	}
	if tokenURL != "https://api.example.com/oauth2/token" {
		t.Fatalf("tokenURL = %q", tokenURL)
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

func TestAuthCodeAuthenticateUsesExplicitContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.Context().Err(); err != nil {
				return nil, err
			}
			return testResponse(200, "application/json", `{"access_token":"unexpected-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			callbackURL, state := mustCallbackURL(t, raw)
			resp, err := http.Get(fmt.Sprintf("%s/?state=%s&code=late-code", callbackURL, url.QueryEscape(state)))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.Authenticate(ctx, req, AuthContext{Params: map[string]string{
		"client_id":     "id1",
		"authorize_url": "https://auth.example.com/authorize",
		"token_url":     "https://auth.example.com/token",
		"redirect_port": availablePort(t),
	}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
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
				resp, err := http.Get(fmt.Sprintf("%s/?state=%s&code=final-code", callbackURL, url.QueryEscape(state)))
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
	var wrongPathStatus atomic.Int32
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
				wrongURL := strings.Replace(callbackURL, "/oauth/callback", "/callback", 1)
				wrongResp, err := http.Get(fmt.Sprintf("%s?state=%s&code=wrong-code", wrongURL, url.QueryEscape(state)))
				if err != nil {
					t.Errorf("wrong callback request failed: %v", err)
					return
				}
				wrongPathStatus.Store(int32(wrongResp.StatusCode))
				wrongResp.Body.Close()
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
		"redirect_path": "/oauth/callback",
	})
	if err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if got := wrongPathStatus.Load(); got != http.StatusNotFound {
		t.Fatalf("/callback status = %d, want %d", got, http.StatusNotFound)
	}
	parsedAuthorize, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	if got := parsedAuthorize.Query().Get("redirect_uri"); !strings.HasSuffix(got, "/oauth/callback") {
		t.Fatalf("authorize redirect_uri = %q, want /oauth/callback", got)
	}
	if got := gotForm.Get("redirect_uri"); !strings.HasSuffix(got, "/oauth/callback") {
		t.Fatalf("token redirect_uri = %q, want /oauth/callback", got)
	}
}

func TestAuthCode_HTTPSCallback(t *testing.T) {
	certPath, keyPath := writeOAuthCallbackCert(t)
	tlsClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	var authorizeURL string
	var gotForm url.Values
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			gotForm = r.Form
			return testResponse(200, "application/json", `{"access_token":"https-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			authorizeURL = raw
			go func() {
				callbackURL, state := mustCallbackURL(t, raw)
				if !strings.HasPrefix(callbackURL, "https://localhost:") {
					t.Errorf("callback URL = %q, want https localhost", callbackURL)
					return
				}
				resp, err := tlsClient.Get(fmt.Sprintf("%s?state=%s&code=secure-code", callbackURL, url.QueryEscape(state)))
				if err != nil {
					t.Errorf("https callback request failed: %v", err)
					return
				}
				resp.Body.Close()
			}()
			return nil
		},
	}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{
		"client_id":       "id1",
		"authorize_url":   "https://auth.example.com/authorize",
		"token_url":       "https://auth.example.com/token",
		"redirect_scheme": "https",
		"redirect_port":   availablePort(t),
		"redirect_path":   "/callback",
		"redirect_cert":   certPath,
		"redirect_key":    keyPath,
	})
	if err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer https-token" {
		t.Fatalf("Authorization = %q, want HTTPS callback token", got)
	}
	parsedAuthorize, err := url.Parse(authorizeURL)
	if err != nil {
		t.Fatalf("parse authorize URL: %v", err)
	}
	redirectURI := parsedAuthorize.Query().Get("redirect_uri")
	if !strings.HasPrefix(redirectURI, "https://localhost:") || !strings.HasSuffix(redirectURI, "/callback") {
		t.Fatalf("authorize redirect_uri = %q, want HTTPS callback URI", redirectURI)
	}
	if got := gotForm.Get("redirect_uri"); got != redirectURI {
		t.Fatalf("token redirect_uri = %q, want %q", got, redirectURI)
	}
	for _, key := range []string{"redirect_scheme", "redirect_port", "redirect_path", "redirect_cert", "redirect_key"} {
		if got := parsedAuthorize.Query().Get(key); got != "" {
			t.Fatalf("%s forwarded to authorize endpoint: %q", key, got)
		}
		if got := gotForm.Get(key); got != "" {
			t.Fatalf("%s forwarded to token endpoint: %q", key, got)
		}
	}
}

func TestOAuthRedirectConfig(t *testing.T) {
	cases := []struct {
		name            string
		params          map[string]string
		requireTLSFiles bool
		wantURI         string
		wantError       string
	}{
		{
			name:    "default",
			params:  map[string]string{},
			wantURI: "http://localhost:8484/",
		},
		{
			name: "https",
			params: map[string]string{
				"redirect_scheme": "https",
				"redirect_port":   "9443",
				"redirect_path":   "/callback",
				"redirect_cert":   "cert.pem",
				"redirect_key":    "key.pem",
			},
			wantURI: "https://localhost:9443/callback",
		},
		{
			name:      "invalid scheme",
			params:    map[string]string{"redirect_scheme": "ftp"},
			wantError: "redirect_scheme must be http or https",
		},
		{
			name:            "https requires cert and key for callback listener",
			params:          map[string]string{"redirect_scheme": "https", "redirect_cert": "cert.pem"},
			requireTLSFiles: true,
			wantError:       "redirect_cert and redirect_key are required",
		},
		{
			name: "https manual URL does not require cert and key",
			params: map[string]string{
				"redirect_scheme": "https",
				"redirect_port":   "9443",
			},
			wantURI: "https://localhost:9443/",
		},
		{
			name:      "invalid path",
			params:    map[string]string{"redirect_path": "callback"},
			wantError: "redirect_path",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := oauthRedirectConfigFromParams(tc.params, tc.requireTLSFiles)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.uri != tc.wantURI {
				t.Fatalf("uri = %q, want %q", got.uri, tc.wantURI)
			}
		})
	}
}

func TestAuthCode_NoBrowserHTTPSDoesNotRequireCallbackCert(t *testing.T) {
	var authorizeURL string
	var gotForm url.Values
	h := &AuthorizationCode{
		NoBrowser: true,
		CanPrompt: true,
		Prompt: func(prompt string) (string, error) {
			return "manual-code", nil
		},
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			gotForm = r.Form
			return testResponse(200, "application/json", `{"access_token":"manual-https-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			authorizeURL = raw
			t.Fatalf("OpenBrowser should not be called in no-browser mode")
			return nil
		},
	}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{
		"client_id":       "id1",
		"authorize_url":   "https://auth.example.com/authorize",
		"token_url":       "https://auth.example.com/token",
		"redirect_scheme": "https",
		"redirect_port":   availablePort(t),
	})
	if err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if authorizeURL != "" {
		t.Fatalf("OpenBrowser captured URL %q", authorizeURL)
	}
	if got := gotForm.Get("redirect_uri"); !strings.HasPrefix(got, "https://localhost:") {
		t.Fatalf("token redirect_uri = %q, want HTTPS localhost", got)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer manual-https-token" {
		t.Fatalf("Authorization = %q, want manual HTTPS token", got)
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
		if body := <-bodyCh; !strings.Contains(body, "Login Successful!") || !strings.Contains(body, `class="check"`) || !strings.Contains(body, "@keyframes success-bg") {
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
		if body := <-bodyCh; !strings.Contains(body, "Authentication failed") || !strings.Contains(body, `class="x"`) || !strings.Contains(body, "@keyframes failure-bg") {
			t.Fatalf("callback body = %q", body)
		}
	})
}

func TestOAuthCallbackErrorPageEscapesDetail(t *testing.T) {
	body := oauthCallbackErrorPage("Authentication failed", `<script>alert("nope")</script>`, "")
	if strings.Contains(body, "<script>") {
		t.Fatalf("callback body includes raw script: %q", body)
	}
	if !strings.Contains(body, `&lt;script&gt;alert(&#34;nope&#34;)&lt;/script&gt;`) {
		t.Fatalf("callback body does not include escaped detail: %q", body)
	}
}

func TestOAuthCallbackPageUsesConfiguredBackgroundColor(t *testing.T) {
	body := oauthCallbackSuccessPage("Login Successful!", "Done.", "#50fa7b")
	if !strings.Contains(body, "to { background: #50fa7b; }") {
		t.Fatalf("callback body does not use configured color: %q", body)
	}
}

func TestOAuthCallbackPageRejectsInvalidBackgroundColor(t *testing.T) {
	body := oauthCallbackErrorPage("Authentication failed", "Nope.", `red; background: url("bad")`)
	if strings.Contains(body, "url(") {
		t.Fatalf("callback body includes invalid CSS color: %q", body)
	}
	if !strings.Contains(body, "to { background: #E94F37; }") {
		t.Fatalf("callback body did not fall back to default failure color: %q", body)
	}
}

func TestOAuthCallbackPagesUseCustomHTMLFields(t *testing.T) {
	h := &AuthorizationCode{
		CallbackSuccessHTML: `<html><body><h1>Welcome to my-tool</h1></body></html>`,
		CallbackErrorHTML:   `<html><body><h1>$ERROR</h1><p>$DETAILS</p></body></html>`,
	}
	if got, want := h.oauthCallbackSuccessPage("Login Successful!", "Done."), h.CallbackSuccessHTML; got != want {
		t.Fatalf("custom success body = %q, want %q", got, want)
	}
	got := h.oauthCallbackErrorPage("Authentication failed", `<script>alert("nope")</script>`)
	want := `<html><body><h1>Authentication failed</h1><p>&lt;script&gt;alert(&#34;nope&#34;)&lt;/script&gt;</p></body></html>`
	if got != want {
		t.Fatalf("custom error body = %q, want %q", got, want)
	}
}

func TestOAuthCallbackPagesUseCustomHTMLParams(t *testing.T) {
	h := &AuthorizationCode{
		CallbackSuccessHTML: `<html>field success</html>`,
		CallbackErrorHTML:   `<html>field error</html>`,
	}
	pages := h.oauthCallbackPages(map[string]string{
		callbackSuccessHTMLParam: `<html><body><h1>$TITLE</h1><p>$DETAILS</p></body></html>`,
		callbackErrorHTMLParam:   `<html><body><h1>$ERROR</h1><p>$DETAILS</p></body></html>`,
	})
	if got, want := pages.successPage("Login Successful!", "Done."), `<html><body><h1>Login Successful!</h1><p>Done.</p></body></html>`; got != want {
		t.Fatalf("custom success body = %q, want %q", got, want)
	}
	got := pages.errorPage("Error: access_denied", `bad <reason>`, "access_denied")
	want := `<html><body><h1>access_denied</h1><p>bad &lt;reason&gt;</p></body></html>`
	if got != want {
		t.Fatalf("custom error body = %q, want %q", got, want)
	}
}

func TestAuthCode_CustomCallbackHTMLParamsAreNotForwarded(t *testing.T) {
	h := &AuthorizationCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue(callbackSuccessHTMLParam); got != "" {
				t.Fatalf("%s forwarded to token endpoint: %q", callbackSuccessHTMLParam, got)
			}
			if got := r.FormValue(callbackErrorHTMLParam); got != "" {
				t.Fatalf("%s forwarded to token endpoint: %q", callbackErrorHTMLParam, got)
			}
			return testResponse(200, "application/json", `{"access_token":"custom-html-token","token_type":"bearer","expires_in":3600}`), nil
		}),
		OpenBrowser: func(raw string) error {
			go func() {
				authorizeURL, err := url.Parse(raw)
				if err != nil {
					t.Errorf("parse authorize URL: %v", err)
					return
				}
				if got := authorizeURL.Query().Get(callbackSuccessHTMLParam); got != "" {
					t.Errorf("%s forwarded to authorize endpoint: %q", callbackSuccessHTMLParam, got)
					return
				}
				if got := authorizeURL.Query().Get(callbackErrorHTMLParam); got != "" {
					t.Errorf("%s forwarded to authorize endpoint: %q", callbackErrorHTMLParam, got)
					return
				}
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
	err := h.OnRequest(req, map[string]string{
		"client_id":              "id1",
		"authorize_url":          "https://auth.example.com/authorize",
		"token_url":              "https://auth.example.com/token",
		"redirect_port":          availablePort(t),
		callbackSuccessHTMLParam: `<html>success</html>`,
		callbackErrorHTMLParam:   `<html>error</html>`,
	})
	if err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer custom-html-token" {
		t.Fatalf("Authorization = %q, want custom HTML token", got)
	}
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
	if runtime.GOOS == "linux" {
		// xdg-open does not support --, so we skip the separator check on Linux.
		// Real OAuth URLs always start with https://, so this is safe in practice.
		t.Skip("xdg-open does not accept --")
	}
	cmd := defaultOpenBrowserCommand("-https://example.com")
	args := strings.Join(cmd.Args, "\x00")
	if !strings.Contains(args, "\x00--\x00-https://example.com") {
		t.Fatalf("browser command should pass -- before URL, got %#v", cmd.Args)
	}
}

func TestDefaultOpenBrowserCommandLinuxUsesXDGOpenWithoutSeparator(t *testing.T) {
	cmd := defaultOpenBrowserCommandForGOOS("linux", "https://example.com/callback?code=abc")
	want := []string{"xdg-open", "https://example.com/callback?code=abc"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("linux browser command args = %#v, want %#v", cmd.Args, want)
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

func writeOAuthCallbackCert(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:   time.Now().Add(-time.Minute),
		NotAfter:    time.Now().Add(time.Hour),
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "callback.pem")
	keyPath := filepath.Join(dir, "callback.key")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}
