package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/cli"
	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/spec"
)

// minimalOpenAPI is the smallest valid OpenAPI 3.1 spec for use in tests.
const minimalOpenAPI = `{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "paths": {}
}`

// newSpecTestCLI returns a CLI whose config points at a temp file containing
// the given API name → base URL mapping, and whose spec cache uses a temp dir.
func newSpecTestCLI(t *testing.T, apiName, baseURL string) *cli.CLI {
	t.Helper()

	// Write a temporary config file.
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			apiName: {BaseURL: baseURL},
		},
	}
	data, _ := json.Marshal(cfg)
	cfgFile := t.TempDir() + "/restish.json"
	if err := writeFile(cfgFile, data); err != nil {
		t.Fatal(err)
	}

	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.ConfigPath = cfgFile
	c.SpecCachePath = t.TempDir()
	c.RetryBaseDelay = 0 // no retry delays in spec tests
	return c
}

// writeFile writes data to path.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// TestSpecDiscoveryViaLinkHeader verifies that the discovery finds a spec URL
// advertised in a Link: <url>; rel="service-desc" response header.
func TestSpecDiscoveryViaLinkHeader(t *testing.T) {
	var specHits atomic.Int32

	specSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		specHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, minimalOpenAPI)
	}))
	t.Cleanup(specSrv.Close)

	baseSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="service-desc"`, specSrv.URL))
		w.WriteHeader(200)
	}))
	t.Cleanup(baseSrv.Close)

	cacheDir := t.TempDir()
	cfg := spec.DiscoverConfig{
		APIName:  "testapi",
		BaseURL:  baseSrv.URL,
		CacheDir: cacheDir,
		Version:  "test",
	}

	apiSpec, err := spec.Discover(context.Background(), cfg, spec.DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if apiSpec == nil {
		t.Fatal("expected spec, got nil")
	}
	if specHits.Load() == 0 {
		t.Error("expected at least one hit to the spec server")
	}
}

// TestSpecDiscoveryViaWellKnownPath verifies that /openapi.json is probed
// during discovery.
func TestSpecDiscoveryViaWellKnownPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, minimalOpenAPI)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := spec.DiscoverConfig{
		APIName:  "testapi",
		BaseURL:  srv.URL,
		CacheDir: t.TempDir(),
		Version:  "test",
	}

	apiSpec, err := spec.Discover(context.Background(), cfg, spec.DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if apiSpec == nil {
		t.Fatal("expected spec, got nil")
	}
}

// TestSpecCacheReusedOnSecondDiscover verifies that a second Discover call does
// not hit the server (result is served from the CBOR cache).
func TestSpecCacheReusedOnSecondDiscover(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, minimalOpenAPI)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cacheDir := t.TempDir()
	cfg := spec.DiscoverConfig{
		APIName:  "testapi",
		BaseURL:  srv.URL,
		CacheDir: cacheDir,
		Version:  "test",
	}

	// First discover: fetches from network.
	s1, err := spec.Discover(context.Background(), cfg, spec.DefaultLoaders())
	if err != nil || s1 == nil {
		t.Fatalf("first Discover failed: err=%v spec=%v", err, s1)
	}
	firstHits := hits.Load()

	// Second discover: should be served from cache.
	s2, err := spec.Discover(context.Background(), cfg, spec.DefaultLoaders())
	if err != nil || s2 == nil {
		t.Fatalf("second Discover failed: err=%v spec=%v", err, s2)
	}
	if hits.Load() != firstHits {
		t.Errorf("expected no additional server hits on second discover; got %d total", hits.Load())
	}
}

// TestAPISync forces a fresh spec fetch after the cache has been primed.
func TestAPISync(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, minimalOpenAPI)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := newSpecTestCLI(t, "myapi", srv.URL)

	// Load config so the CLI knows about the API.
	if err := runCLI(c, "restish", "api", "sync", "myapi"); err != nil {
		t.Fatalf("api sync failed: %v", err)
	}
	firstHits := hits.Load()
	if firstHits == 0 {
		t.Error("expected at least one server hit during api sync")
	}

	// A second sync should hit the server again (cache was invalidated by sync itself).
	if err := runCLI(c, "restish", "api", "sync", "myapi"); err != nil {
		t.Fatalf("second api sync failed: %v", err)
	}
	if hits.Load() <= firstHits {
		t.Errorf("expected second api sync to hit server again; hits before=%d after=%d", firstHits, hits.Load())
	}
}

// TestSpecUnknownContentTypeDoesNotCrash verifies that a non-spec response at
// a well-known path is gracefully ignored.
func TestSpecUnknownContentTypeDoesNotCrash(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html>not a spec</html>")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfg := spec.DiscoverConfig{
		APIName:  "testapi",
		BaseURL:  srv.URL,
		CacheDir: t.TempDir(),
		Version:  "test",
	}

	// Discovery should fail gracefully (no panic, just "not found").
	apiSpec, _ := spec.Discover(context.Background(), cfg, spec.DefaultLoaders())
	// nil spec is the expected outcome — no crash is the key assertion.
	_ = apiSpec
}

// runCLI is a helper that loads the CLI's config and runs a command.
func runCLI(c *cli.CLI, args ...string) error {
	return c.Run(args)
}
