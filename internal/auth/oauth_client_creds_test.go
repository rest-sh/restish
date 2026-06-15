package auth

import (
	"github.com/rest-sh/restish/v2/auth"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTokenClient(t *testing.T, callCount *atomic.Int32, accessToken string, expiresIn int) *http.Client {
	t.Helper()
	return testHTTPClient(func(r *http.Request) (*http.Response, error) {
		callCount.Add(1)
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Fatalf("expected form content type, got %q", ct)
		}
		if accept := r.Header.Get("Accept"); accept != "application/json" {
			t.Fatalf("expected Accept application/json, got %q", accept)
		}
		return testResponse(200, "application/json", fmt.Sprintf(`{"access_token":%q,"token_type":"bearer","expires_in":%d}`, accessToken, expiresIn)), nil
	})
}

func TestClientCredentials_FetchesToken(t *testing.T) {
	var count atomic.Int32
	client := newTokenClient(t, &count, "access-abc", 3600)

	h := &ClientCredentials{
		Cache:      auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.json")),
		HTTPClient: client,
	}
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "https://auth.example.com/token",
		"_cache_key":    "myapi:default",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer access-abc" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer access-abc")
	}
}

func TestClientCredentialsResolvesRelativeTokenURL(t *testing.T) {
	var gotTokenURL string
	h := &ClientCredentials{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			gotTokenURL = r.URL.String()
			return testResponse(200, "application/json", `{"access_token":"relative-token","token_type":"bearer","expires_in":3600}`), nil
		}),
	}
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	err := h.OnRequest(req, map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "oauth2/token",
		"_base_url":     "https://api.example.com/v1",
	})
	if err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if gotTokenURL != "https://api.example.com/v1/oauth2/token" {
		t.Fatalf("token URL = %q", gotTokenURL)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer relative-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestClientCredentialsRejectsInvalidDirectTokenEndpoint(t *testing.T) {
	h := &ClientCredentials{}
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	err := h.OnRequest(req, map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "http://auth.example.com/token",
	})
	if err == nil {
		t.Fatal("expected invalid token_url error")
	}
	if !strings.Contains(err.Error(), "must use https") {
		t.Fatalf("expected https error, got %v", err)
	}
}

func TestClientCredentials_CachesToken(t *testing.T) {
	var count atomic.Int32
	client := newTokenClient(t, &count, "cached-token", 3600)

	cache := auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "https://auth.example.com/token",
		"_cache_key":    "myapi:default",
	}

	h := &ClientCredentials{Cache: cache, HTTPClient: client}
	req1, _ := http.NewRequest("GET", "https://api.example.com", nil)
	if err := h.OnRequest(req1, params); err != nil {
		t.Fatalf("first request: %v", err)
	}

	req2, _ := http.NewRequest("GET", "https://api.example.com", nil)
	if err := h.OnRequest(req2, params); err != nil {
		t.Fatalf("second request: %v", err)
	}

	if n := count.Load(); n != 1 {
		t.Errorf("expected token endpoint to be called once, got %d", n)
	}
	if got := req2.Header.Get("Authorization"); got != "Bearer cached-token" {
		t.Errorf("second request Authorization: got %q, want %q", got, "Bearer cached-token")
	}
}

func TestClientCredentials_ExpiredToken_Refetches(t *testing.T) {
	var count atomic.Int32
	client := newTokenClient(t, &count, "fresh-token", 3600)

	cache := auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	cacheKey := "myapi:default"
	_ = cache.Set(cacheKey, auth.CachedToken{
		AccessToken: "old-token",
		Expiry:      time.Now().Add(-time.Hour),
	})

	h := &ClientCredentials{Cache: cache, HTTPClient: client}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "https://auth.example.com/token",
		"_cache_key":    cacheKey,
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer fresh-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer fresh-token")
	}
	if n := count.Load(); n != 1 {
		t.Errorf("expected token endpoint called once, got %d", n)
	}
}

func TestClientCredentials_OIDCDiscovery(t *testing.T) {
	var count atomic.Int32
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://auth.example.com/.well-known/openid-configuration":
			return testResponse(200, "application/json", `{"token_endpoint":"https://auth.example.com/token"}`), nil
		case "https://auth.example.com/token":
			count.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("grant_type"); got != "client_credentials" {
				t.Fatalf("grant_type = %q", got)
			}
			return testResponse(200, "application/json", `{"access_token":"oidc-token","token_type":"bearer","expires_in":3600}`), nil
		default:
			t.Fatalf("unexpected URL %q", r.URL.String())
			return nil, nil
		}
	})

	h := &ClientCredentials{
		Cache:      auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.json")),
		HTTPClient: client,
	}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"issuer_url":    "https://auth.example.com",
		"_cache_key":    "myapi:default",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer oidc-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer oidc-token")
	}
	if n := count.Load(); n != 1 {
		t.Errorf("expected token endpoint called once, got %d", n)
	}
}

func TestClientCredentials_SendsExpectedFormFields(t *testing.T) {
	var got url.Values
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		got = r.Form
		return testResponse(200, "application/json", `{"access_token":"abc","token_type":"bearer","expires_in":3600}`), nil
	})

	h := &ClientCredentials{HTTPClient: client}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "https://auth.example.com/token",
		"scopes":        "read write",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Get("client_id") != "id1" || got.Get("client_secret") != "sec1" || got.Get("scope") != "read write" {
		t.Fatalf("unexpected form values: %#v", got)
	}
}

func TestClientCredentials_AuthMethodClientSecretBasic(t *testing.T) {
	var got url.Values
	var authz string
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		authz = r.Header.Get("Authorization")
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		got = r.Form
		return testResponse(200, "application/json", `{"access_token":"abc","token_type":"bearer","expires_in":3600}`), nil
	})

	h := &ClientCredentials{HTTPClient: client}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "https://auth.example.com/token",
		"auth_method":   "client_secret_basic",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authz == "" {
		t.Fatal("expected Authorization header on token request")
	}
	if got.Get("client_secret") != "" {
		t.Fatalf("client_secret should not be in form for basic auth: %#v", got)
	}
	if got.Get("client_id") != "id1" {
		t.Fatalf("client_id = %q", got.Get("client_id"))
	}
}

func TestClientCredentials_PassesThroughExtraTokenParams(t *testing.T) {
	var got url.Values
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		got = r.Form
		return testResponse(200, "application/json", `{"access_token":"abc","token_type":"bearer","expires_in":3600}`), nil
	})

	h := &ClientCredentials{HTTPClient: client}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     "https://auth.example.com/token",
		"audience":      "https://api.example.com/",
		"resource":      "urn:example",
		"organization":  "acme",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Get("audience") != "https://api.example.com/" || got.Get("resource") != "urn:example" || got.Get("organization") != "acme" {
		t.Fatalf("unexpected passthrough form values: %#v", got)
	}
}
