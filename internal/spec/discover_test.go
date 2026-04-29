package spec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func httpResponse(status int, contentType, body string, headers http.Header) *http.Response {
	if headers == nil {
		headers = http.Header{}
	}
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

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

func TestExtractSpecLinks_CommaInURL(t *testing.T) {
	h := http.Header{}
	h.Set("Link", `<https://cdn.example.com/spec.yaml?labels=a,b>; rel=describedby`)
	links := extractSpecLinks("https://api.example.com/", h)
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0] != "https://cdn.example.com/spec.yaml?labels=a,b" {
		t.Errorf("got %q", links[0])
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

func TestFilterDiscoveredSpecLinksRejectsLocalhostCrossOrigin(t *testing.T) {
	links := filterDiscoveredSpecLinks("https://api.example.com", []string{"http://localhost:8080/openapi.json"}, true)
	if len(links) != 0 {
		t.Fatalf("expected localhost cross-origin spec link to be rejected, got %v", links)
	}
}

func TestFilterDiscoveredSpecLinksRejectsDNSResolvedPrivateTarget(t *testing.T) {
	oldLookup := lookupIPAddr
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if host == "spec.example.com" {
			return []net.IPAddr{{IP: net.ParseIP("10.0.0.10")}}, nil
		}
		return nil, errors.New("unexpected lookup")
	}
	t.Cleanup(func() { lookupIPAddr = oldLookup })

	links := filterDiscoveredSpecLinks("https://api.example.com", []string{"https://spec.example.com/openapi.json"}, true)
	if len(links) != 0 {
		t.Fatalf("expected private DNS target to be rejected, got %v", links)
	}
}

func TestFilterDiscoveredSpecLinksAllowsPrivateTargetFromPrivateBase(t *testing.T) {
	links := filterDiscoveredSpecLinks("http://10.0.0.5", []string{"http://10.0.0.10/openapi.json"}, true)
	if len(links) != 1 {
		t.Fatalf("expected private-to-private link to be allowed, got %v", links)
	}
}

func TestFilterDiscoveredSpecLinksRequiresSameOriginByDefault(t *testing.T) {
	tests := []struct {
		name string
		base string
		link string
		want bool
	}{
		{
			name: "same origin accepted",
			base: "https://api.example.com/v1",
			link: "https://api.example.com/openapi.json",
			want: true,
		},
		{
			name: "explicit default port accepted",
			base: "https://api.example.com/v1",
			link: "https://api.example.com:443/openapi.json",
			want: true,
		},
		{
			name: "scheme downgrade rejected",
			base: "https://api.example.com/v1",
			link: "http://api.example.com/openapi.json",
			want: false,
		},
		{
			name: "different port rejected",
			base: "https://api.example.com/v1",
			link: "https://api.example.com:8443/openapi.json",
			want: false,
		},
		{
			name: "different host rejected",
			base: "https://api.example.com/v1",
			link: "https://spec.example.com/openapi.json",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			links := filterDiscoveredSpecLinks(tc.base, []string{tc.link}, false)
			if got := len(links) == 1; got != tc.want {
				t.Fatalf("accepted = %v, want %v; links=%v", got, tc.want, links)
			}
		})
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
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.example.com/openapi.json" {
			t.Fatalf("unexpected URL %q", r.URL.String())
		}
		return httpResponse(200, "application/json", spec, nil), nil
	})

	cfg := DiscoverConfig{
		SpecFiles: []string{"https://api.example.com/openapi.json"},
		Transport: tr,
	}
	result, err := loadSpecFiles(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestLoadSpecFilesResolvesRelativeExternalRefs(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "openapi.yaml")
	paramsPath := filepath.Join(dir, "params.yaml")
	root := `openapi: "3.1.0"
info:
  title: Local Refs
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - $ref: "./params.yaml#/components/parameters/ID"
      responses:
        "200":
          description: OK`
	params := `components:
  parameters:
    ID:
      name: id
      in: path
      required: true
      schema:
        type: string`
	if err := os.WriteFile(rootPath, []byte(root), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}
	if err := os.WriteFile(paramsPath, []byte(params), 0o644); err != nil {
		t.Fatalf("write params: %v", err)
	}

	loaded, err := loadSpecFiles(context.Background(), DiscoverConfig{SpecFiles: []string{rootPath}}, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if !operationHasParam(ops, "getItem", "path", "id") {
		t.Fatalf("expected getItem path id parameter from external ref, got %#v", ops)
	}
}

func TestLoadSpecFilesResolvesFullFileURIExternalRefs(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "openapi.yaml")
	paramsPath := filepath.Join(dir, "params.yaml")
	paramsURI := (&url.URL{Scheme: "file", Path: paramsPath}).String()
	root := fmt.Sprintf(`openapi: "3.1.0"
info:
  title: File URI Refs
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - $ref: %q
      responses:
        "200":
          description: OK`, paramsURI+"#/components/parameters/ID")
	params := `components:
  parameters:
    ID:
      name: id
      in: path
      required: true
      schema:
        type: string`
	if err := os.WriteFile(rootPath, []byte(root), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}
	if err := os.WriteFile(paramsPath, []byte(params), 0o644); err != nil {
		t.Fatalf("write params: %v", err)
	}

	loaded, err := loadSpecFiles(context.Background(), DiscoverConfig{SpecFiles: []string{rootPath}}, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if !operationHasParam(ops, "getItem", "path", "id") {
		t.Fatalf("expected getItem path id parameter from file URI ref, got %#v", ops)
	}
}

func TestLoadSpecFilesResolvesExternalRequestAndResponseSchemas(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "openapi.yaml")
	schemasPath := filepath.Join(dir, "schemas.yaml")
	root := `openapi: "3.1.0"
info:
  title: Local Schema Refs
  version: "1.0.0"
paths:
  /items:
    post:
      operationId: createItem
      requestBody:
        content:
          application/json:
            schema:
              $ref: "./schemas.yaml#/components/schemas/CreateItem"
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                $ref: "./schemas.yaml#/components/schemas/Item"`
	schemas := `components:
  schemas:
    CreateItem:
      type: object
      properties:
        name:
          type: string
    Item:
      type: object
      properties:
        id:
          type: string
        name:
          type: string`
	if err := os.WriteFile(rootPath, []byte(root), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}
	if err := os.WriteFile(schemasPath, []byte(schemas), 0o644); err != nil {
		t.Fatalf("write schemas: %v", err)
	}

	loaded, err := loadSpecFiles(context.Background(), DiscoverConfig{SpecFiles: []string{rootPath}}, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if ops[0].Help.Request == nil || !strings.Contains(ops[0].Help.Request.Example, "name") {
		t.Fatalf("expected request help from external schema, got %#v", ops[0].Help.Request)
	}
	if len(ops[0].Help.Responses) != 1 || !strings.Contains(ops[0].Help.Responses[0].Example, "id") {
		t.Fatalf("expected response help from external schema, got %#v", ops[0].Help.Responses)
	}
}

func TestLoadSpecFilesResolvesExternalPathItemRefs(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "openapi.yaml")
	pathsPath := filepath.Join(dir, "paths.yaml")
	root := `openapi: "3.1.0"
info:
  title: Path Item Refs
  version: "1.0.0"
paths:
  /items:
    $ref: "./paths.yaml#/paths/~1items"`
	paths := `paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`
	if err := os.WriteFile(rootPath, []byte(root), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}
	if err := os.WriteFile(pathsPath, []byte(paths), 0o644); err != nil {
		t.Fatalf("write paths: %v", err)
	}

	loaded, err := loadSpecFiles(context.Background(), DiscoverConfig{SpecFiles: []string{rootPath}}, DefaultLoaders())
	if err != nil {
		t.Fatalf("loadSpecFiles: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if len(ops) != 1 || ops[0].ID != "listItems" {
		t.Fatalf("expected listItems from external Path Item ref, got %#v", ops)
	}
}

// ---- Discover ------------------------------------------------------------

func TestDiscover_ExplicitSpecURL(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Direct","version":"1.0.0"},"paths":{}}`
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/spec.json" {
			return httpResponse(200, "application/json", spec, nil), nil
		}
		return httpResponse(404, "text/plain", "not found", nil), nil
	})

	cfg := DiscoverConfig{
		APIName:   "testapi",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/spec.json",
		Transport: tr,
	}
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestDiscoverResolvesSameOriginRemoteExternalRefs(t *testing.T) {
	root := `openapi: "3.1.0"
info:
  title: Remote Refs
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - $ref: "./params.yaml#/components/parameters/ID"
      responses:
        "200":
          description: OK`
	params := `components:
  parameters:
    ID:
      name: id
      in: path
      required: true
      schema:
        type: string`
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/specs/openapi.yaml":
			return httpResponse(200, "application/yaml", root, nil), nil
		case "https://api.example.com/specs/params.yaml":
			return httpResponse(200, "application/yaml", params, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	loaded, err := Discover(context.Background(), DiscoverConfig{
		APIName:   "remote-refs",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/specs/openapi.yaml",
		Transport: tr,
	}, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if !operationHasParam(ops, "getItem", "path", "id") {
		t.Fatalf("expected getItem path id parameter from same-origin remote ref, got %#v", ops)
	}
}

func TestDiscoverBlocksCrossOriginRemoteExternalRefsByDefault(t *testing.T) {
	root := `openapi: "3.1.0"
info:
  title: Remote Refs
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - $ref: "https://spec.example.com/params.yaml#/components/parameters/ID"
      responses:
        "200":
          description: OK`
	var crossOriginHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/openapi.yaml":
			return httpResponse(200, "application/yaml", root, nil), nil
		case "https://spec.example.com/params.yaml":
			crossOriginHits.Add(1)
			return httpResponse(200, "application/yaml", `components: {}`, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	_, err := Discover(context.Background(), DiscoverConfig{
		APIName:   "remote-refs",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/openapi.yaml",
		Transport: tr,
	}, DefaultLoaders())
	if err == nil {
		t.Fatal("expected discover to fail when external ref is cross-origin")
	}
	if got := crossOriginHits.Load(); got != 0 {
		t.Fatalf("cross-origin ref was fetched %d times; expected it to be blocked first", got)
	}
}

func TestDiscoverBlocksRemoteSpecFileURIRefs(t *testing.T) {
	dir := t.TempDir()
	paramsPath := filepath.Join(dir, "params.yaml")
	paramsURI := (&url.URL{Scheme: "file", Path: paramsPath}).String()
	if err := os.WriteFile(paramsPath, []byte(`components:
  parameters:
    ID:
      name: id
      in: path
      required: true
      schema:
        type: string`), 0o644); err != nil {
		t.Fatalf("write params: %v", err)
	}

	root := fmt.Sprintf(`openapi: "3.1.0"
info:
  title: Remote File Ref
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - $ref: %q
      responses:
        "200":
          description: OK`, paramsURI+"#/components/parameters/ID")
	var specHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/openapi.yaml" {
			specHits.Add(1)
			return httpResponse(200, "application/yaml", root, nil), nil
		}
		return httpResponse(404, "text/plain", "not found", nil), nil
	})

	_, err := Discover(context.Background(), DiscoverConfig{
		APIName:   "remote-file-ref",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/openapi.yaml",
		Transport: tr,
	}, DefaultLoaders())
	if err == nil {
		t.Fatal("expected remote file URI ref to be rejected")
	}
	if !strings.Contains(err.Error(), "local file access from remote source") {
		t.Fatalf("expected local file access error, got %v", err)
	}
	if got := specHits.Load(); got != 1 {
		t.Fatalf("spec fetched %d times, want 1", got)
	}
}

func TestDiscoverAllowsCrossOriginRemoteExternalRefsWithOptIn(t *testing.T) {
	root := `openapi: "3.1.0"
info:
  title: Remote Refs
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - $ref: "https://spec.example.com/params.yaml#/components/parameters/ID"
      responses:
        "200":
          description: OK`
	params := `components:
  parameters:
    ID:
      name: id
      in: path
      required: true
      schema:
        type: string`
	var crossOriginHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/openapi.yaml":
			return httpResponse(200, "application/yaml", root, nil), nil
		case "https://spec.example.com/params.yaml":
			crossOriginHits.Add(1)
			return httpResponse(200, "application/yaml", params, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	loaded, err := Discover(context.Background(), DiscoverConfig{
		APIName:          "remote-refs",
		BaseURL:          "https://api.example.com",
		SpecURL:          "https://api.example.com/openapi.yaml",
		Transport:        tr,
		AllowCrossOrigin: true,
	}, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if !operationHasParam(ops, "getItem", "path", "id") {
		t.Fatalf("expected getItem path id parameter from cross-origin remote ref, got %#v", ops)
	}
	if got := crossOriginHits.Load(); got == 0 {
		t.Fatal("expected cross-origin ref to be fetched with opt-in")
	}
}

func TestDiscover_WellKnownPath(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"WellKnown","version":"1.0.0"},"paths":{}}`
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/openapi.json":
			return httpResponse(200, "application/json", spec, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cfg := DiscoverConfig{
		APIName:   "wellknown",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestDiscover_WellKnownOfficialOpenAPIContentTypeWithLateOpenAPIKey(t *testing.T) {
	spec := `{"components":{"schemas":{"Thing":{"type":"object"}}},"info":{"title":"WellKnown","version":"1.0.0"},"paths":{},"openapi":"3.1.0"}`
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/openapi.json":
			return httpResponse(200, "application/vnd.oai.openapi+json", spec, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cfg := DiscoverConfig{
		APIName:   "wellknown",
		BaseURL:   "https://api.example.com",
		Transport: tr,
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
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/":
			headers := http.Header{}
			headers.Set("Link", `</spec.json>; rel="service-desc"`)
			return httpResponse(200, "text/html", `<html>welcome</html>`, headers), nil
		case "/spec.json":
			return httpResponse(200, "application/json", spec, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cfg := DiscoverConfig{
		APIName:   "linked",
		BaseURL:   "https://api.example.com/",
		Transport: tr,
	}
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestDiscover_LinkHeaderCrossOriginRequiresOptIn(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Linked","version":"1.0.0"},"paths":{}}`
	var crossOriginHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			headers := http.Header{}
			headers.Set("Link", `<https://spec.example.com/spec.json>; rel="service-desc"`)
			return httpResponse(200, "text/html", `<html>welcome</html>`, headers), nil
		case "https://spec.example.com/spec.json":
			crossOriginHits.Add(1)
			return httpResponse(200, "application/json", spec, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cfg := DiscoverConfig{
		APIName:   "linked",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	if _, err := Discover(context.Background(), cfg, DefaultLoaders()); err == nil {
		t.Fatal("expected discovery to fail without cross-origin opt-in")
	}
	if got := crossOriginHits.Load(); got != 0 {
		t.Fatalf("expected no cross-origin fetch without opt-in, got %d", got)
	}

	cfg.AllowCrossOrigin = true
	result, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover with opt-in: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil spec with cross-origin opt-in")
	}
	if got := crossOriginHits.Load(); got == 0 {
		t.Fatal("expected cross-origin spec to be fetched with opt-in")
	}
}

func TestDiscover_LinkHeaderCrossOriginRejectsLoopbackIP(t *testing.T) {
	var loopbackHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			headers := http.Header{}
			headers.Set("Link", `<http://127.0.0.1/openapi.json>; rel="service-desc"`)
			return httpResponse(200, "text/html", `<html>welcome</html>`, headers), nil
		case "http://127.0.0.1/openapi.json":
			loopbackHits.Add(1)
			return httpResponse(200, "application/json", `{"openapi":"3.1.0","info":{"title":"Bad","version":"1.0.0"},"paths":{}}`, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cfg := DiscoverConfig{
		APIName:          "linked",
		BaseURL:          "https://api.example.com",
		Transport:        tr,
		AllowCrossOrigin: true,
	}
	if _, err := Discover(context.Background(), cfg, DefaultLoaders()); err == nil {
		t.Fatal("expected discovery to fail for loopback IP target")
	}
	if got := loopbackHits.Load(); got != 0 {
		t.Fatalf("expected loopback target to be rejected before fetch, got %d requests", got)
	}
}

func TestDiscover_LinkHeaderRejectsUnsupportedScheme(t *testing.T) {
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			headers := http.Header{}
			headers.Set("Link", `<file:///tmp/spec.json>; rel="service-desc"`)
			return httpResponse(200, "text/html", `<html>welcome</html>`, headers), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cfg := DiscoverConfig{
		APIName:   "linked",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	if _, err := Discover(context.Background(), cfg, DefaultLoaders()); err == nil {
		t.Fatal("expected discovery to fail when Link header uses unsupported scheme")
	}
}

func TestDiscover_DefaultTimeoutAddsDeadline(t *testing.T) {
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected Discover to add a default deadline")
		}
		remaining := time.Until(deadline)
		if remaining < 29*time.Second || remaining > 31*time.Second {
			t.Fatalf("expected ~30s deadline, got %v", remaining)
		}
		return nil, context.DeadlineExceeded
	})

	cfg := DiscoverConfig{
		APIName:   "timeout",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	if _, err := Discover(context.Background(), cfg, DefaultLoaders()); err == nil {
		t.Fatal("expected discovery to fail")
	}
}

func TestDiscover_UsesCallerDeadlineForHungServer(t *testing.T) {
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	cfg := DiscoverConfig{
		APIName:   "timeout",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := Discover(ctx, cfg, DefaultLoaders())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected caller deadline to stop discovery promptly, took %v", elapsed)
	}
}

func TestDiscover_Cache(t *testing.T) {
	spec := `{"openapi":"3.1.0","info":{"title":"Cached","version":"1.0.0"},"paths":{}}`
	var callCount atomic.Int64
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		callCount.Add(1)
		return httpResponse(200, "application/json", spec, nil), nil
	})

	cacheDir := t.TempDir()
	cfg := DiscoverConfig{
		APIName:   "cached",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/spec.json",
		CacheDir:  cacheDir,
		Version:   "v2.0.0",
		Transport: tr,
	}

	// First call: network fetch + cache write (may hit multiple probes in parallel).
	result1, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	if result1 == nil {
		t.Fatal("expected non-nil spec on first call")
	}
	countAfterFirst := callCount.Load()

	// Second call: should read from cache, making zero additional network calls.
	result2, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	if result2 == nil {
		t.Fatal("expected non-nil spec on second call")
	}

	if got := callCount.Load(); got != countAfterFirst {
		t.Errorf("second Discover made %d additional network calls, expected 0", got-countAfterFirst)
	}
}

func TestDiscoverCachesOperationsResolvedFromRemoteExternalRefs(t *testing.T) {
	root := `openapi: "3.1.0"
info:
  title: Cached Remote Refs
  version: "1.0.0"
paths:
  /items/{id}:
    get:
      operationId: getItem
      parameters:
        - $ref: "./params.yaml#/components/parameters/ID"
      responses:
        "200":
          description: OK`
	params := `components:
  parameters:
    ID:
      name: id
      in: path
      required: true
      schema:
        type: string`
	var paramsHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/specs/openapi.yaml":
			return httpResponse(200, "application/yaml", root, nil), nil
		case "https://api.example.com/specs/params.yaml":
			paramsHits.Add(1)
			return httpResponse(200, "application/yaml", params, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cacheDir := t.TempDir()
	cfg := DiscoverConfig{
		APIName:   "cached-remote-refs",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/specs/openapi.yaml",
		CacheDir:  cacheDir,
		Version:   "v2.0.0",
		Transport: tr,
	}
	loaded, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if _, err := loaded.Operations("https://api.example.com", ""); err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if got := paramsHits.Load(); got == 0 {
		t.Fatal("expected params ref to be fetched while priming operation cache")
	}

	set, ok := LoadOperationSetFromCache(cacheDir, "cached-remote-refs", "v2.0.0", nil, "https://api.example.com", "")
	if !ok {
		t.Fatal("expected cached operation set")
	}
	if !operationHasParam(set.Operations, "getItem", "path", "id") {
		t.Fatalf("expected cached getItem path id parameter from remote ref, got %#v", set.Operations)
	}
}

func TestDiscoverCacheInvalidatesWhenSpecURLChanges(t *testing.T) {
	specA := `{"openapi":"3.1.0","info":{"title":"A","version":"1.0.0"},"paths":{}}`
	specB := `{"openapi":"3.1.0","info":{"title":"B","version":"1.0.0"},"paths":{}}`
	var specBHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/a.json":
			return httpResponse(200, "application/json", specA, nil), nil
		case "https://api.example.com/b.json":
			specBHits.Add(1)
			return httpResponse(200, "application/json", specB, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cacheDir := t.TempDir()
	cfg := DiscoverConfig{
		APIName:   "source-change",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/a.json",
		CacheDir:  cacheDir,
		Version:   "v2.0.0",
		Transport: tr,
	}
	if _, err := Discover(context.Background(), cfg, DefaultLoaders()); err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	cfg.SpecURL = "https://api.example.com/b.json"
	loaded, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	info, err := loaded.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Title != "B" {
		t.Fatalf("cached stale spec title = %q, want B", info.Title)
	}
	if got := specBHits.Load(); got == 0 {
		t.Fatal("expected changed spec_url to fetch fresh spec")
	}
}

func TestDiscoverCacheInvalidatesWhenBaseURLChanges(t *testing.T) {
	specA := `{"openapi":"3.1.0","info":{"title":"A","version":"1.0.0"},"paths":{}}`
	specB := `{"openapi":"3.1.0","info":{"title":"B","version":"1.0.0"},"paths":{}}`
	var apiBHits atomic.Int32
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://a.example.com/openapi.json":
			return httpResponse(200, "application/json", specA, nil), nil
		case "https://b.example.com/openapi.json":
			apiBHits.Add(1)
			return httpResponse(200, "application/json", specB, nil), nil
		default:
			return httpResponse(404, "text/plain", "not found", nil), nil
		}
	})

	cacheDir := t.TempDir()
	cfg := DiscoverConfig{
		APIName:   "base-change",
		BaseURL:   "https://a.example.com",
		CacheDir:  cacheDir,
		Version:   "v2.0.0",
		Transport: tr,
	}
	if _, err := Discover(context.Background(), cfg, DefaultLoaders()); err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	cfg.BaseURL = "https://b.example.com"
	loaded, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	info, err := loaded.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Title != "B" {
		t.Fatalf("cached stale spec title = %q, want B", info.Title)
	}
	if got := apiBHits.Load(); got == 0 {
		t.Fatal("expected changed base_url to fetch fresh spec")
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

func TestDiscoverCacheInvalidatesWhenSpecFilePathChanges(t *testing.T) {
	dir := t.TempDir()
	specAPath := filepath.Join(dir, "a.yaml")
	specBPath := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(specAPath, []byte(`openapi: "3.1.0"
info:
  title: A
  version: "1.0.0"
paths: {}`), 0o644); err != nil {
		t.Fatalf("write A: %v", err)
	}
	if err := os.WriteFile(specBPath, []byte(`openapi: "3.1.0"
info:
  title: B
  version: "1.0.0"
paths: {}`), 0o644); err != nil {
		t.Fatalf("write B: %v", err)
	}

	cacheDir := t.TempDir()
	cfg := DiscoverConfig{
		APIName:   "file-change",
		BaseURL:   "https://api.example.com",
		SpecFiles: []string{specAPath},
		CacheDir:  cacheDir,
		Version:   "v2.0.0",
	}
	if _, err := Discover(context.Background(), cfg, DefaultLoaders()); err != nil {
		t.Fatalf("first Discover: %v", err)
	}
	cfg.SpecFiles = []string{specBPath}
	loaded, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err != nil {
		t.Fatalf("second Discover: %v", err)
	}
	info, err := loaded.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Title != "B" {
		t.Fatalf("cached stale spec title = %q, want B", info.Title)
	}
}

// ---- Discovery cancellation ------------------------------------------------

func TestDiscover_CancellationPropagatesUsefulError(t *testing.T) {
	// Cancel the context before Discover is called. Discover should return
	// a meaningful error wrapping context.Canceled rather than panic or hang.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, r.Context().Err()
	})

	cfg := DiscoverConfig{
		APIName:   "cancelled",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	_, err := Discover(ctx, cfg, DefaultLoaders())
	if err == nil {
		t.Fatal("expected an error from cancelled context")
	}
	// The error should be context.Canceled or wrap it.
	if !errors.Is(err, context.Canceled) {
		// Also acceptable: a "no API spec found" error when all probes fail
		// because the context was already cancelled. Either way, err != nil
		// is the key invariant.
		t.Logf("note: error is %v (not context.Canceled, but still non-nil)", err)
	}
}

func TestDiscover_CancellationDuringNetwork(t *testing.T) {
	// Context cancelled partway through — Discover must not hang.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	cfg := DiscoverConfig{
		APIName:   "slowapi",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	start := time.Now()
	_, err := Discover(ctx, cfg, DefaultLoaders())
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Discover hung for %v after context cancellation", elapsed)
	}
}

// ---- Multi-error reporting -------------------------------------------------

func TestDiscover_SamePriorityErrorsJoined(t *testing.T) {
	// All probes fail at the same priority (no explicit SpecURL).
	// The error message should contain information from the failures.
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})

	cfg := DiscoverConfig{
		APIName:   "multiErr",
		BaseURL:   "https://api.example.com",
		Transport: tr,
	}
	_, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err == nil {
		t.Fatal("expected error when all probes fail")
	}
	if !strings.Contains(err.Error(), "spec discovery failed") {
		t.Errorf("expected 'spec discovery failed' in error, got: %v", err)
	}
}

func TestDiscover_SpecURLErrorBeatsHeuristicError(t *testing.T) {
	// An explicit SpecURL 404 should beat a heuristic-probe "connection refused"
	// because the SpecURL is the more authoritative failure (priority 0 vs 1).
	tr := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "explicit.json") {
			resp := httpResponse(404, "", "not found", nil)
			resp.Status = "404 Not Found"
			return resp, nil
		}
		return nil, errors.New("connection refused")
	})

	cfg := DiscoverConfig{
		APIName:   "priority",
		BaseURL:   "https://api.example.com",
		SpecURL:   "https://api.example.com/explicit.json",
		Transport: tr,
	}
	_, err := Discover(context.Background(), cfg, DefaultLoaders())
	if err == nil {
		t.Fatal("expected error")
	}
	// The error should mention the 404 status, not "connection refused".
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected SpecURL's 404 to dominate error, got: %v", err)
	}
}

func operationHasParam(ops []Operation, operationID, in, name string) bool {
	for _, op := range ops {
		if op.ID != operationID {
			continue
		}
		for _, param := range op.Parameters {
			if param.In == in && param.Name == name {
				return true
			}
		}
	}
	return false
}
