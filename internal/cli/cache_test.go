package cli_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// newCacheableServer starts a test server that counts how many times it is
// called and responds with a cacheable JSON body.
func newCacheableServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestCacheSecondRequestServedFromCache verifies that the first GET hits the
// server but the second is served from cache (server call count stays at 1).
func TestCacheSecondRequestServedFromCache(t *testing.T) {
	var hits atomic.Int32
	srv := newCacheableServer(t, &hits)
	cacheDir := t.TempDir()

	c1, out1, _ := newTestCLI(t)
	c1.Hooks().CachePath = cacheDir
	if err := c1.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	if !strings.Contains(out1.String(), "ok") {
		t.Errorf("expected body in first response, got: %q", out1.String())
	}

	c2, out2, _ := newTestCLI(t)
	c2.Hooks().CachePath = cacheDir
	if err := c2.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	if !strings.Contains(out2.String(), "ok") {
		t.Errorf("expected body in second response, got: %q", out2.String())
	}

	if n := hits.Load(); n != 1 {
		t.Errorf("expected server to be called once, got %d calls", n)
	}
}

func TestCacheAuthenticatedRequestBypassesCache(t *testing.T) {
	var hits atomic.Int32
	srv := newCacheableServer(t, &hits)
	cacheDir := t.TempDir()
	cfgFile := t.TempDir() + "/restish.json"
	cfgData, _ := json.Marshal(map[string]any{
		"apis": map[string]any{
			"svc": map[string]any{
				"base_url": srv.URL,
				"profiles": map[string]any{
					"default": map[string]any{
						"headers": []string{"Authorization: Bearer secret"},
					},
				},
			},
		},
	})
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	for i := range 2 {
		c, _, _ := newTestCLI(t)
		c.Hooks().ConfigPath = cfgFile
		c.Hooks().CachePath = cacheDir
		if err := c.Run([]string{"restish", "get", "svc/items"}); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}

	if n := hits.Load(); n != 2 {
		t.Fatalf("authenticated responses should bypass cache, got %d server hits", n)
	}
}

// TestCacheNoCacheBypassesCache verifies that --rsh-no-cache always hits the
// server regardless of a primed cache.
func TestCacheNoCacheBypassesCache(t *testing.T) {
	var hits atomic.Int32
	srv := newCacheableServer(t, &hits)
	cacheDir := t.TempDir()

	for i := range 3 {
		c, _, _ := newTestCLI(t)
		c.Hooks().CachePath = cacheDir
		if err := c.Run([]string{"restish", "get", "--rsh-no-cache", srv.URL}); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}

	if n := hits.Load(); n != 3 {
		t.Errorf("--rsh-no-cache: expected 3 server hits, got %d", n)
	}
}

// TestCacheClearEmptiesCache verifies that "restish cache clear" removes all
// cached responses, causing the next request to hit the server again.
func TestCacheClearEmptiesCache(t *testing.T) {
	var hits atomic.Int32
	srv := newCacheableServer(t, &hits)
	cacheDir := t.TempDir()

	// Prime the cache.
	c1, _, _ := newTestCLI(t)
	c1.Hooks().CachePath = cacheDir
	if err := c1.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("prime request failed: %v", err)
	}

	// Clear the cache.
	c2, out2, _ := newTestCLI(t)
	c2.Hooks().CachePath = cacheDir
	if err := c2.Run([]string{"restish", "cache", "clear"}); err != nil {
		t.Fatalf("cache clear failed: %v", err)
	}
	if !strings.Contains(out2.String(), "cleared") {
		t.Errorf("expected cleared message, got: %q", out2.String())
	}

	// Next request should hit the server again (cache is empty).
	c3, _, _ := newTestCLI(t)
	c3.Hooks().CachePath = cacheDir
	if err := c3.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("post-clear request failed: %v", err)
	}

	if n := hits.Load(); n != 2 {
		t.Errorf("expected 2 server hits (prime + post-clear), got %d", n)
	}
}

// TestCacheNoStoreNotCached verifies that responses with Cache-Control:
// no-store are not stored in the cache.
func TestCacheNoStoreNotCached(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"secret":true}`))
	}))
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	for i := range 2 {
		c, _, _ := newTestCLI(t)
		c.Hooks().CachePath = cacheDir
		if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
	}

	if n := hits.Load(); n != 2 {
		t.Errorf("Cache-Control: no-store: expected 2 server hits, got %d", n)
	}
}

// TestCacheInfo verifies that "restish cache info" prints cache statistics.
func TestCacheInfo(t *testing.T) {
	var hits atomic.Int32
	srv := newCacheableServer(t, &hits)
	cacheDir := t.TempDir()

	// Prime the cache with one entry.
	c1, _, _ := newTestCLI(t)
	c1.Hooks().CachePath = cacheDir
	if err := c1.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("prime request failed: %v", err)
	}

	c2, out, _ := newTestCLI(t)
	c2.Hooks().CachePath = cacheDir
	if err := c2.Run([]string{"restish", "cache", "info"}); err != nil {
		t.Fatalf("cache info failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"Directory:", "Size:", "Entries:"} {
		if !strings.Contains(got, want) {
			t.Errorf("cache info output missing %q:\n%s", want, got)
		}
	}
}
