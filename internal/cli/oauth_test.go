package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/auth"
)

func oauthTokenResponse(r *http.Request, accessToken string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"access_token":"` + accessToken + `","token_type":"bearer","expires_in":3600}`,
		)),
		Request: r,
	}
}

// TestOAuthClientCredentials_BearerHeader verifies that an API configured with
// oauth-client-credentials adds the correct Authorization: Bearer header.
func TestOAuthClientCredentials_BearerHeader(t *testing.T) {
	var tokenCount atomic.Int32
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://oauth.example.com/token":
			tokenCount.Add(1)
			return oauthTokenResponse(r, "cc-token"), nil
		case "https://api.example.com/items":
			rr.capture(r)
			return jsonResponse(200, `{}`), nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"token_url": "https://oauth.example.com/token"
							}
						}
					}
				}
			}
		}
	}`
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
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://oauth.example.com/token":
			tokenCount.Add(1)
			return oauthTokenResponse(r, "cached-cc-token"), nil
		case "https://api.example.com/items":
			return jsonResponse(200, `{}`), nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"token_url": "https://oauth.example.com/token"
							}
						}
					}
				}
			}
		}
	}`

	cacheFile := filepath.Join(t.TempDir(), "tokens.json")

	// First request.
	c1, _, _ := newTestCLI()
	c1.ConfigPath = writeAPIConfig(t, cfg)
	c1.TokenCachePath = cacheFile
	useTransport(c1, transport)
	if err := c1.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("first request: %v", err)
	}

	// Second request (new CLI instance, same cache file).
	c2, _, _ := newTestCLI()
	c2.ConfigPath = c1.ConfigPath
	c2.TokenCachePath = cacheFile
	useTransport(c2, transport)
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
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://issuer.example.com/.well-known/openid-configuration":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"token_endpoint":"https://issuer.example.com/token"}`)),
				Request:    r,
			}, nil
		case "https://issuer.example.com/token":
			tokenCount.Add(1)
			return oauthTokenResponse(r, "oidc-cc-token"), nil
		case "https://api.example.com/items":
			rr.capture(r)
			return jsonResponse(200, `{}`), nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"issuer_url": "https://issuer.example.com"
							}
						}
					}
				}
			}
		}
	}`
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
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://oauth.example.com/token":
			tokenCount.Add(1)
			return oauthTokenResponse(r, "new-token"), nil
		case "https://api.example.com/items":
			rr.capture(r)
			return jsonResponse(200, `{}`), nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"token_url": "https://oauth.example.com/token"
							}
						}
					}
				}
			}
		}
	}`

	cacheFile := filepath.Join(t.TempDir(), "tokens.json")

	// Pre-populate cache with expired token.
	tc := auth.NewTokenCache(cacheFile)
	_ = tc.Set("myapi:default", auth.CachedToken{
		AccessToken: "old-expired-token",
		Expiry:      time.Now().Add(-time.Hour),
	})

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
	c, out, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = cacheFile

	if err := c.Run([]string{"restish", "api", "clear-auth-cache", "myapi"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `profile "default"`) {
		t.Fatalf("expected current profile in output, got %q", out.String())
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

func TestClearAuthCache_AllProfiles(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "tokens.json")
	tc := auth.NewTokenCache(cacheFile)
	_ = tc.Set("myapi:default", auth.CachedToken{AccessToken: "tok1"})
	_ = tc.Set("myapi:prod", auth.CachedToken{AccessToken: "tok2"})
	_ = tc.Set("other:default", auth.CachedToken{AccessToken: "tok3"})

	cfg := `{"apis": {"myapi": {"base_url": "https://api.example.com"}}}`
	c, out, _ := newTestCLI()
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = cacheFile

	if err := c.Run([]string{"restish", "api", "clear-auth-cache", "--all", "myapi"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "all profiles") {
		t.Fatalf("expected all profiles output, got %q", out.String())
	}
	data, err := readJSONFile(cacheFile)
	if err != nil {
		t.Fatalf("reading cache file: %v", err)
	}
	if _, ok := data["other:default"]; !ok {
		t.Fatal("expected unrelated cache entry to remain")
	}
	for _, key := range []string{"myapi:default", "myapi:prod"} {
		if _, ok := data[key]; ok {
			t.Fatalf("expected %q to be deleted", key)
		}
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

func TestOAuthAuthorizationCode_NoBrowserManualCodeFallback(t *testing.T) {
	var rr requestRecorder
	c, _, errBuf := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://oauth.example.com/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("code"); got != "manual-code" {
				t.Fatalf("code = %q", got)
			}
			return oauthTokenResponse(r, "manual-token"), nil
		case "https://api.example.com/items":
			rr.capture(r)
			return jsonResponse(200, `{}`), nil
		default:
			return jsonResponse(404, `{"error":"not found"}`), nil
		}
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-authorization-code",
							"params": {
								"client_id": "myid",
								"authorize_url": "https://oauth.example.com/authorize",
								"token_url": "https://oauth.example.com/token"
							}
						}
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = filepath.Join(t.TempDir(), "tokens.json")
	c.PassReader = strings.NewReader("manual-code\n")

	if err := c.Run([]string{"restish", "--rsh-no-browser", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("Authorization"); got != "Bearer manual-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if !strings.Contains(errBuf.String(), "Paste the authorization code") {
		t.Fatalf("expected manual code prompt, got %q", errBuf.String())
	}
}

func TestOAuthDeviceCodeFlow(t *testing.T) {
	var rr requestRecorder
	c, _, errBuf := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://oauth.example.com/device":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(`{
					"device_code":"device-123",
					"user_code":"ABCD-EFGH",
					"verification_uri":"https://verify.example.com",
					"verification_uri_complete":"https://verify.example.com/complete",
					"interval":1,
					"expires_in":60
				}`)),
				Request: r,
			}, nil
		case "https://oauth.example.com/token":
			return oauthTokenResponse(r, "device-token"), nil
		case "https://api.example.com/items":
			rr.capture(r)
			return jsonResponse(200, `{}`), nil
		default:
			return jsonResponse(404, `{"error":"not found"}`), nil
		}
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-device-code",
							"params": {
								"client_id": "myid",
								"device_authorization_url": "https://oauth.example.com/device",
								"token_url": "https://oauth.example.com/token"
							}
						}
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = filepath.Join(t.TempDir(), "tokens.json")

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("Authorization"); got != "Bearer device-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if !strings.Contains(errBuf.String(), "verify.example.com") {
		t.Fatalf("expected verification instructions on stderr, got %q", errBuf.String())
	}
}

func TestOAuthClientCredentials_401RetryForcesFreshToken(t *testing.T) {
	var tokenCount atomic.Int32
	var apiCount atomic.Int32
	var rr requestRecorder
	c, _, _ := newTestCLI()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://oauth.example.com/token":
			if tokenCount.Add(1) == 1 {
				return oauthTokenResponse(r, "stale-token"), nil
			}
			return oauthTokenResponse(r, "fresh-token"), nil
		case "https://api.example.com/items":
			rr.capture(r)
			if apiCount.Add(1) == 1 {
				return &http.Response{
					StatusCode: 401,
					Proto:      "HTTP/1.1",
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"error":"expired"}`)),
					Request:    r,
				}, nil
			}
			return jsonResponse(200, `{}`), nil
		default:
			return jsonResponse(404, `{"error":"not found"}`), nil
		}
	})

	cfg := `{
		"apis": {
			"myapi": {
				"base_url": "https://api.example.com",
				"profiles": {
					"default": {
						"auth": {
							"type": "oauth-client-credentials",
							"params": {
								"client_id": "myid",
								"client_secret": "mysecret",
								"token_url": "https://oauth.example.com/token"
							}
						}
					}
				}
			}
		}
	}`
	c.ConfigPath = writeAPIConfig(t, cfg)
	c.TokenCachePath = filepath.Join(t.TempDir(), "tokens.json")

	if err := c.Run([]string{"restish", "get", "myapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokenCount.Load() != 2 {
		t.Fatalf("expected token endpoint called twice, got %d", tokenCount.Load())
	}
	if apiCount.Load() != 2 {
		t.Fatalf("expected API called twice, got %d", apiCount.Load())
	}
	if got := rr.Last().Header.Get("Authorization"); got != "Bearer fresh-token" {
		t.Fatalf("Authorization = %q", got)
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
