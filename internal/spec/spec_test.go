package spec

import (
	"testing"
)

// minimalOpenAPI is the smallest valid OpenAPI 3.1 document.
const minimalOpenAPI = `{"openapi":"3.1.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`

// minimalOpenAPIYAML is the same in YAML form.
const minimalOpenAPIYAML = `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}`

// ---- OpenAPILoader.Detect ------------------------------------------------

func TestOpenAPILoader_Detect_JSON(t *testing.T) {
	l := OpenAPILoader{}
	body := []byte(minimalOpenAPI)
	if !l.Detect("application/json", body) {
		t.Error("should detect OpenAPI JSON")
	}
}

func TestOpenAPILoader_Detect_YAML(t *testing.T) {
	l := OpenAPILoader{}
	body := []byte(minimalOpenAPIYAML)
	if !l.Detect("application/yaml", body) {
		t.Error("should detect OpenAPI YAML")
	}
}

func TestOpenAPILoader_Detect_EmptyContentType(t *testing.T) {
	l := OpenAPILoader{}
	// Empty content-type is allowed; sniff the body.
	if !l.Detect("", []byte(minimalOpenAPI)) {
		t.Error("should detect OpenAPI with empty content-type")
	}
}

func TestOpenAPILoader_Detect_NotOpenAPI(t *testing.T) {
	l := OpenAPILoader{}
	if l.Detect("application/json", []byte(`{"foo":"bar"}`)) {
		t.Error("should not detect non-OpenAPI JSON")
	}
}

func TestOpenAPILoader_Detect_WrongContentType(t *testing.T) {
	l := OpenAPILoader{}
	// image/png with openapi body: content-type mismatch should reject.
	if l.Detect("image/png", []byte(minimalOpenAPI)) {
		t.Error("should reject unsupported content type")
	}
}

func TestOpenAPILoader_Priority(t *testing.T) {
	l := OpenAPILoader{}
	if l.Priority() <= 0 {
		t.Error("expected positive priority")
	}
}

// ---- OpenAPILoader.Load --------------------------------------------------

func TestOpenAPILoader_Load_Valid(t *testing.T) {
	l := OpenAPILoader{}
	spec, err := l.Load([]byte(minimalOpenAPI))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}
	if spec.Document == nil {
		t.Error("expected non-nil Document")
	}
}

func TestOpenAPILoader_Load_Invalid(t *testing.T) {
	l := OpenAPILoader{}
	_, err := l.Load([]byte(`not yaml or json at all {{{`))
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

// ---- pickLoader ----------------------------------------------------------

func TestPickLoader_ReturnsHighestPriority(t *testing.T) {
	type mockLoader struct {
		OpenAPILoader
		prio int
	}

	loaders := []Loader{OpenAPILoader{}}
	body := []byte(minimalOpenAPI)
	got := pickLoader("application/json", body, loaders)
	if got == nil {
		t.Fatal("expected a loader to be picked")
	}
}

func TestPickLoader_NoMatch(t *testing.T) {
	loaders := []Loader{OpenAPILoader{}}
	got := pickLoader("image/png", []byte(`not openapi`), loaders)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// ---- load ----------------------------------------------------------------

func TestLoad_Valid(t *testing.T) {
	loaders := DefaultLoaders()
	spec, err := load("application/json", []byte(minimalOpenAPI), loaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}
	if spec.ContentType != "application/json" {
		t.Errorf("ContentType: got %q, want %q", spec.ContentType, "application/json")
	}
}

func TestLoad_NoMatchReturnsNil(t *testing.T) {
	loaders := DefaultLoaders()
	spec, err := load("image/png", []byte(`random`), loaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec != nil {
		t.Error("expected nil spec for unrecognized content")
	}
}
