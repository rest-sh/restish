package cli_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
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
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().RetryBaseDelay = 0 // no retry delays in spec tests
	return c
}

// writeFile writes data to path.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// TestSpecDiscoveryViaLinkHeader verifies that same-host Link discovery works
// without opt-in.
func TestSpecDiscoveryViaLinkHeader(t *testing.T) {
	var specHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Link": []string{`<https://api.example.com/openapi.json>; rel="service-desc"`}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case "https://api.example.com/openapi.json":
			specHits.Add(1)
			return jsonResponse(200, minimalOpenAPI), nil
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

	cacheDir := t.TempDir()
	cfg := spec.DiscoverConfig{
		APIName:   "testapi",
		BaseURL:   "https://api.example.com",
		CacheDir:  cacheDir,
		Version:   "test",
		Transport: tr,
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

func TestSpecDiscoveryViaCrossOriginLinkRequiresOptIn(t *testing.T) {
	var specHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Link": []string{`<https://spec.example.com/openapi.json>; rel="service-desc"`}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case "https://spec.example.com/openapi.json":
			specHits.Add(1)
			return jsonResponse(200, minimalOpenAPI), nil
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

	cfg := spec.DiscoverConfig{
		APIName:   "testapi",
		BaseURL:   "https://api.example.com",
		CacheDir:  t.TempDir(),
		Version:   "test",
		Transport: tr,
	}
	if _, err := spec.Discover(context.Background(), cfg, spec.DefaultLoaders()); err == nil {
		t.Fatal("expected cross-origin link discovery to fail without opt-in")
	}
	if got := specHits.Load(); got != 0 {
		t.Fatalf("expected no cross-origin fetch without opt-in, got %d", got)
	}

	cfg.AllowCrossOrigin = true
	apiSpec, err := spec.Discover(context.Background(), cfg, spec.DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover with opt-in failed: %v", err)
	}
	if apiSpec == nil {
		t.Fatal("expected spec, got nil")
	}
}

// TestSpecDiscoveryViaWellKnownPath verifies that /openapi.json is probed
// during discovery.
func TestSpecDiscoveryViaWellKnownPath(t *testing.T) {
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/openapi.json":
			return jsonResponse(200, minimalOpenAPI), nil
		default:
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
	})

	cfg := spec.DiscoverConfig{
		APIName:   "testapi",
		BaseURL:   "https://api.example.com",
		CacheDir:  t.TempDir(),
		Version:   "test",
		Transport: tr,
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
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/openapi.json":
			hits.Add(1)
			return jsonResponse(200, minimalOpenAPI), nil
		default:
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
	})

	cacheDir := t.TempDir()
	cfg := spec.DiscoverConfig{
		APIName:   "testapi",
		BaseURL:   "https://api.example.com",
		CacheDir:  cacheDir,
		Version:   "test",
		Transport: tr,
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
	c := newSpecTestCLI(t, "myapi", "https://api.example.com")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/openapi.json":
			hits.Add(1)
			return jsonResponse(200, minimalOpenAPI), nil
		default:
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
	})

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
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"text/html"}},
				Body:       io.NopCloser(strings.NewReader("<html>not a spec</html>")),
				Request:    r,
			}, nil
		default:
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
	})

	cfg := spec.DiscoverConfig{
		APIName:   "testapi",
		BaseURL:   "https://api.example.com",
		CacheDir:  t.TempDir(),
		Version:   "test",
		Transport: tr,
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
