package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// newTokenServer returns an httptest.Server that responds to POST requests with
// a JSON token response. callCount is incremented on each request.
func newTokenServer(t *testing.T, callCount *atomic.Int32, accessToken string, expiresIn int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		resp := tokenResponse{
			AccessToken: accessToken,
			TokenType:   "bearer",
			ExpiresIn:   expiresIn,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClientCredentials_FetchesToken(t *testing.T) {
	var count atomic.Int32
	srv := newTokenServer(t, &count, "access-abc", 3600)

	h := &ClientCredentials{
		Cache: NewTokenCache(filepath.Join(t.TempDir(), "tokens.json")),
	}
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     srv.URL,
		"_cache_key":    "myapi:default",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer access-abc" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer access-abc")
	}
}

func TestClientCredentials_CachesToken(t *testing.T) {
	var count atomic.Int32
	srv := newTokenServer(t, &count, "cached-token", 3600)

	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     srv.URL,
		"_cache_key":    "myapi:default",
	}

	// First call: should hit the token endpoint.
	h := &ClientCredentials{Cache: cache}
	req1, _ := http.NewRequest("GET", "https://api.example.com", nil)
	if err := h.OnRequest(req1, params); err != nil {
		t.Fatalf("first request: %v", err)
	}

	// Second call: should use the cached token.
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
	srv := newTokenServer(t, &count, "fresh-token", 3600)

	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	cacheKey := "myapi:default"

	// Pre-populate cache with an expired token.
	_ = cache.Set(cacheKey, CachedToken{
		AccessToken: "old-token",
		Expiry:      time.Now().Add(-time.Hour), // already expired
	})

	h := &ClientCredentials{Cache: cache}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"token_url":     srv.URL,
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
	// Token server.
	var count atomic.Int32
	tokenSrv := newTokenServer(t, &count, "oidc-token", 3600)

	// OIDC discovery server.
	discoverySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"token_endpoint":%q}`, tokenSrv.URL)
	}))
	t.Cleanup(discoverySrv.Close)

	h := &ClientCredentials{
		Cache: NewTokenCache(filepath.Join(t.TempDir(), "tokens.json")),
	}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":     "id1",
		"client_secret": "sec1",
		"issuer_url":    discoverySrv.URL,
		"_cache_key":    "myapi:default",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer oidc-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer oidc-token")
	}
}
