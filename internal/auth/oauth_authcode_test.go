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

// TestAuthCode_RefreshToken verifies that an expired cached token with a
// refresh token causes the handler to exchange the refresh token for a new
// access token without opening a browser.
func TestAuthCode_RefreshToken(t *testing.T) {
	var refreshCallCount atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", 400)
			return
		}
		if gt := r.FormValue("grant_type"); gt != "refresh_token" {
			http.Error(w, fmt.Sprintf("unexpected grant_type %q", gt), 400)
			return
		}
		refreshCallCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		resp := tokenResponse{AccessToken: "refreshed-token", TokenType: "bearer", ExpiresIn: 3600}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(tokenSrv.Close)

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
		Cache: cache,
		OpenBrowser: func(url string) error {
			t.Errorf("OpenBrowser called unexpectedly with %q", url)
			return nil
		},
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":  "id1",
		"token_url":  tokenSrv.URL,
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
