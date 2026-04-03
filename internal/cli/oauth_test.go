package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/auth"
)

// newOAuthTokenServer starts a test token server that responds with the given
// access token. callCount is incremented on each request.
func newOAuthTokenServer(t *testing.T, callCount *atomic.Int32, accessToken string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":%q,"token_type":"bearer","expires_in":3600}`, accessToken)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestOAuthClientCredentials_BearerHeader verifies that an API configured with
// oauth-client-credentials adds the correct Authorization: Bearer header.
func TestOAuthClientCredentials_BearerHeader(t *testing.T) {
	var tokenCount atomic.Int32
	tokenSrv := newOAuthTokenServer(t, &tokenCount, "cc-token")

	var rr requestRecorder
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr.capture(r)
		w.WriteHeader(200)
	}))
	t.Cleanup(apiSrv.Close)

	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"token_url": %q
							}
						}
					}
				}
			}
		}
	}`, apiSrv.URL, tokenSrv.URL)

	c, _, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = filepath.Join(t.TempDir(), "tokens.json")

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("Authorization"); got != "Bearer cc-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer cc-token")
	}
}

// TestOAuthClientCredentials_TokenCached verifies that repeated requests reuse
// the cached token (token endpoint called only once).
func TestOAuthClientCredentials_TokenCached(t *testing.T) {
	var tokenCount atomic.Int32
	tokenSrv := newOAuthTokenServer(t, &tokenCount, "cached-cc-token")

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	t.Cleanup(apiSrv.Close)

	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"token_url": %q
							}
						}
					}
				}
			}
		}
	}`, apiSrv.URL, tokenSrv.URL)

	cacheFile := filepath.Join(t.TempDir(), "tokens.json")

	// First request.
	c1, _, _ := newTestCLI()
	c1.ConfigPath = writeAPIConfig(t, cfg)
	c1.TokenCachePath = cacheFile
	if err := c1.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("first request: %v", err)
	}

	// Second request (new CLI instance, same cache file).
	c2, _, _ := newTestCLI()
	c2.ConfigPath = c1.ConfigPath
	c2.TokenCachePath = cacheFile
	if err := c2.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("second request: %v", err)
	}

	if n := tokenCount.Load(); n != 1 {
		t.Errorf("expected token endpoint called once across two requests, got %d", n)
	}
}

// TestOAuthClientCredentials_OIDCDiscovery verifies that setting issuer_url
// causes the CLI to discover the token endpoint via OIDC discovery.
func TestOAuthClientCredentials_OIDCDiscovery(t *testing.T) {
	var tokenCount atomic.Int32
	tokenSrv := newOAuthTokenServer(t, &tokenCount, "oidc-cc-token")

	discoverySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"token_endpoint":%q}`, tokenSrv.URL)
	}))
	t.Cleanup(discoverySrv.Close)

	var rr requestRecorder
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr.capture(r)
		w.WriteHeader(200)
	}))
	t.Cleanup(apiSrv.Close)

	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"issuer_url": %q
							}
						}
					}
				}
			}
		}
	}`, apiSrv.URL, discoverySrv.URL)

	c, _, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = filepath.Join(t.TempDir(), "tokens.json")

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("Authorization"); got != "Bearer oidc-cc-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer oidc-cc-token")
	}
}

// TestOAuthExpiredToken_RefetchesToken verifies that an expired cached token
// causes the handler to re-fetch a new one from the token endpoint.
func TestOAuthExpiredToken_RefetchesToken(t *testing.T) {
	var tokenCount atomic.Int32
	tokenSrv := newOAuthTokenServer(t, &tokenCount, "new-token")

	var rr requestRecorder
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr.capture(r)
		w.WriteHeader(200)
	}))
	t.Cleanup(apiSrv.Close)

	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"token_url": %q
							}
						}
					}
				}
			}
		}
	}`, apiSrv.URL, tokenSrv.URL)

	cacheFile := filepath.Join(t.TempDir(), "tokens.json")

	// Pre-populate cache with expired token.
	tc := auth.NewTokenCache(cacheFile)
	_ = tc.Set("myapi:default", auth.CachedToken{
		AccessToken: "old-expired-token",
		Expiry:      time.Now().Add(-time.Hour),
	})

	c, _, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = cacheFile

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("Authorization"); got != "Bearer new-token" {
		t.Errorf("Authorization: got %q, want %q", got, "Bearer new-token")
	}
	if n := tokenCount.Load(); n != 1 {
		t.Errorf("expected token endpoint called once, got %d", n)
	}
}

// TestClearAuthCache_RemovesEntry verifies that "api clear-auth-cache <name>"
// deletes the cached token for the named API.
func TestClearAuthCache_RemovesEntry(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "tokens.json")
	tc := auth.NewTokenCache(cacheFile)
	_ = tc.Set("myapi:default", auth.CachedToken{AccessToken: "tok"})

	cfg := `{"apis": {"myapi": {"base_url": "https://api.example.com"}}}`
	c, _, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = cacheFile

	if err := c.Run([]string{"restish", "api", "clear-auth-cache", "myapi"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read the cache file directly and verify the key is gone.
	data, err := readJSONFile(cacheFile)
	if err != nil {
		t.Fatalf("reading cache file: %v", err)
	}
	if _, ok := data["myapi:default"]; ok {
		t.Error("expected cache entry to be deleted, but it still exists")
	}
}

// TestClearAuthCache_UnknownAPI verifies that clearing the cache for an
// unregistered API returns an error.
func TestClearAuthCache_UnknownAPI(t *testing.T) {
	cfg := `{"apis": {}}`
	c, _, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = filepath.Join(t.TempDir(), "tokens.json")

	if err := c.Run([]string{"restish", "api", "clear-auth-cache", "noapi"}); err == nil {
		t.Fatal("expected error for unknown API, got nil")
	}
}

// readJSONFile reads a JSON file and returns it as a map.
func readJSONFile(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	return m, json.Unmarshal(data, &m)
}
