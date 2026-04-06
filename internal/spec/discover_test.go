package spec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// ---- deepMerge -----------------------------------------------------------

func TestDeepMerge_Empty(t *testing.T) {
	result := deepMerge(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestDeepMerge_BaseOnly(t *testing.T) {
	base := map[string]any{"a": 1}
	result := deepMerge(base, nil)
	if result["a"] != 1 {
		t.Errorf("expected a=1, got %v", result["a"])
	}
}

func TestDeepMerge_OverlayWins(t *testing.T) {
	base := map[string]any{"a": 1, "b": 2}
	overlay := map[string]any{"b": 99, "c": 3}
	result := deepMerge(base, overlay)
	if result["a"] != 1 {
		t.Errorf("a: got %v, want 1", result["a"])
	}
	if result["b"] != 99 {
		t.Errorf("b: got %v, want 99", result["b"])
	}
	if result["c"] != 3 {
		t.Errorf("c: got %v, want 3", result["c"])
	}
}

func TestDeepMerge_RecursiveMaps(t *testing.T) {
	base := map[string]any{
		"info": map[string]any{"title": "Old", "version": "1.0"},
	}
	overlay := map[string]any{
		"info": map[string]any{"title": "New"},
	}
	result := deepMerge(base, overlay)
	info, ok := result["info"].(map[string]any)
	if !ok {
		t.Fatalf("info is not a map: %T", result["info"])
	}
	if info["title"] != "New" {
		t.Errorf("title: got %v, want New", info["title"])
	}
	// "version" should be preserved from base.
	if info["version"] != "1.0" {
		t.Errorf("version: got %v, want 1.0", info["version"])
	}
}

func TestDeepMerge_DoesNotMutateInputs(t *testing.T) {
	base := map[string]any{"x": 1}
	overlay := map[string]any{"x": 2}
	result := deepMerge(base, overlay)
	if base["x"] != 1 {
		t.Error("deepMerge mutated base")
	}
	_ = result
}

// ---- joinURL -------------------------------------------------------------

func TestJoinURL(t *testing.T) {
	tests := []struct {
		base, path, want string
	}{
		{"https://api.example.com", "/openapi.json", "https://api.example.com/openapi.json"},
		{"https://api.example.com/", "/openapi.json", "https://api.example.com/openapi.json"},
		{"https://api.example.com/v1", "/openapi.yaml", "https://api.example.com/v1/openapi.yaml"},
	}
	for _, tc := range tests {
		got := joinURL(tc.base, tc.path)
		if got != tc.want {
			t.Errorf("joinURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
	}
}

// ---- isLocalPath ---------------------------------------------------------

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"file:///tmp/spec.yaml", true},
		{"/tmp/spec.yaml", true},
		{"./spec.yaml", true},
		{"spec.yaml", true},
		{"https://api.example.com/spec", false},
		{"http://localhost:8080/openapi.json", false},
	}
	for _, tc := range tests {
		got := isLocalPath(tc.s)
		if got != tc.want {
			t.Errorf("isLocalPath(%q) = %v, want %v", tc.s, got, tc.want)
		}
	}
}

// ---- resolveRef ----------------------------------------------------------

func TestResolveRef_RelativeToBase(t *testing.T) {
	got := resolveRef("https://api.example.com/v1", "/openapi.json")
	want := "https://api.example.com/openapi.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveRef_AbsoluteRef(t *testing.T) {
	got := resolveRef("https://api.example.com/v1", "https://cdn.example.com/spec.json")
	want := "https://cdn.example.com/spec.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---- extractSpecLinks ----------------------------------------------------

func TestExtractSpecLinks_ServiceDesc(t *testing.T) {
	h := http.Header{}
	h.Set("Link", `</openapi.json>; rel="service-desc"`)
	links := extractSpecLinks("https://api.example.com/", h)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0] != "https://api.example.com/openapi.json" {
		t.Errorf("got %q, want %q", links[0], "https://api.example.com/openapi.json")
	}
}

func TestExtractSpecLinks_DescribedBy(t *testing.T) {
	h := http.Header{}
	h.Set("Link", `<https://cdn.example.com/spec.yaml>; rel=describedby`)
	links := extractSpecLinks("https://api.example.com/", h)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0] != "https://cdn.example.com/spec.yaml" {
		t.Errorf("got %q, want %q", links[0], "https://cdn.example.com/spec.yaml")
	}
}

func TestExtractSpecLinks_NoRelevantLinks(t *testing.T) {
	h := http.Header{}
	h.Set("Link", `</next>; rel="next"`)
	links := extractSpecLinks("https://api.example.com/", h)
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d: %v", len(links), links)
	}
}

func TestExtractSpecLinks_NoHeader(t *testing.T) {
	links := extractSpecLinks("https://api.example.com/", http.Header{})
	if len(links) != 0 {
		t.Errorf("expected 0 links, got %d", len(links))
	}
}

// ---- cacheTTL ------------------------------------------------------------

func TestCacheTTL_MaxAge(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Cache-Control", "public, max-age=3600")
	ttl := cacheTTL(resp)
	if ttl.Seconds() != 3600 {
		t.Errorf("expected 3600s, got %v", ttl)
	}
}

func TestCacheTTL_NoHeader(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	ttl := cacheTTL(resp)
	if ttl != 0 {
		t.Errorf("expected 0, got %v", ttl)
	}
}

func TestCacheTTL_NoMaxAge(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Cache-Control", "no-cache, no-store")
	ttl := cacheTTL(resp)
	if ttl != 0 {
		t.Errorf("expected 0, got %v", ttl)
	}
}

// ---- loadSpecFiles -------------------------------------------------------

func TestLoadSpecFiles_LocalFile(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.yaml")
	spec := `openapi: "3.1.0"
info:
  title: Local
  version: "1.0.0"
paths: {}`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfg := DiscoverConfig{
		SpecFiles: []string{specPath},
	}
	result, err := loadSpecFiles(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestLoadSpecFiles_MergesMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	base := `openapi: "3.1.0"
info:
  title: Base
  version: "1.0.0"
paths: {}
x-base: true`
	overlay := `openapi: "3.1.0"
info:
  title: Overlay
  version: "1.0.0"
paths: {}
x-overlay: true`

	basePath := filepath.Join(dir, "base.yaml")
	overlayPath := filepath.Join(dir, "overlay.yaml")
	os.WriteFile(basePath, []byte(base), 0o644)
	os.WriteFile(overlayPath, []byte(overlay), 0o644)

	cfg := DiscoverConfig{SpecFiles: []string{basePath, overlayPath}}
	result, err := loadSpecFiles(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestLoadSpecFiles_MissingFile(t *testing.T) {
	cfg := DiscoverConfig{
		SpecFiles: []string{"/nonexistent/spec.yaml"},
	}
	_, err := loadSpecFiles(context.Background(), cfg, DefaultLoaders())
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadSpecFiles_NetworkSource(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Network","version":"1.0.0"},"paths":{}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(spec))
	}))
	defer srv.Close()

	cfg := DiscoverConfig{
		SpecFiles: []string{srv.URL + "/openapi.json"},
		Transport: srv.Client().Transport,
	}
	result, err := loadSpecFiles(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

// ---- Discover ------------------------------------------------------------

func TestDiscover_ExplicitSpecURL(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Direct","version":"1.0.0"},"paths":{}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(spec))
	}))
	defer srv.Close()

	cfg := DiscoverConfig{
		APIName:   "testapi",
		BaseURL:   srv.URL,
		SpecURL:   srv.URL + "/spec.json",
		Transport: srv.Client().Transport,
	}
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestDiscover_WellKnownPath(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"WellKnown","version":"1.0.0"},"paths":{}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(spec))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := DiscoverConfig{
		APIName:   "wellknown",
		BaseURL:   srv.URL,
		Transport: srv.Client().Transport,
	}
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestDiscover_LinkHeader(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Linked","version":"1.0.0"},"paths":{}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Link", `</spec.json>; rel="service-desc"`)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html>welcome</html>`))
		case "/spec.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(spec))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := DiscoverConfig{
		APIName:   "linked",
		BaseURL:   srv.URL + "/",
		Transport: srv.Client().Transport,
	}
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestDiscover_Cache(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Cached","version":"1.0.0"},"paths":{}}`
	var callCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(spec))
	}))
	defer srv.Close()

	cacheDir := t.TempDir()
	cfg := DiscoverConfig{
		APIName:   "cached",
		BaseURL:   srv.URL,
		SpecURL:   srv.URL + "/spec.json",
		CacheDir:  cacheDir,
		Version:   "v2.0.0",
		Transport: srv.Client().Transport,
	}

	// First call: network fetch + cache write (may hit multiple probes in parallel).
	result1, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	if result1 == nil {
		t.Fatal("expected non-nil spec on first call")
	}
	countAfterFirst := callCount

	// Second call: should read from cache, making zero additional network calls.
	result2, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	if result2 == nil {
		t.Fatal("expected non-nil spec on second call")
	}

	if callCount != countAfterFirst {
		t.Errorf("second Discover made %d additional network calls, expected 0", callCount-countAfterFirst)
	}
}

func TestDiscover_SpecFiles(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.yaml")
	spec := `openapi: "3.1.0"
info:
  title: File
  version: "1.0.0"
paths: {}`
	os.WriteFile(specPath, []byte(spec), 0o644)

	cfg := DiscoverConfig{
		APIName:   "fileapi",
		BaseURL:   "https://api.example.com",
		SpecFiles: []string{specPath},
	}
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}
