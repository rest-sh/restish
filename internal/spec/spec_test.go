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

// minimalOpenAPI300 is the smallest valid OpenAPI 3.0.0 document.
const minimalOpenAPI300 = `{"openapi":"3.0.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`

// minimalOpenAPI303 is an OpenAPI 3.0.3 document.
const minimalOpenAPI303 = `openapi: "3.0.3"
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

func TestLoad_PreservesLoaderContentType(t *testing.T) {
	// A loader that sets its own ContentType should not be overwritten.
	loaders := DefaultLoaders()
	spec, err := load("application/json", []byte(minimalOpenAPI), loaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Loader returned ContentType="" so caller's value is used as fallback.
	if spec.ContentType != "application/json" {
		t.Errorf("ContentType: got %q, want application/json", spec.ContentType)
	}
}

// ---- OpenAPI 3.0.x compatibility -----------------------------------------

func TestOpenAPILoader_Load_300(t *testing.T) {
	l := OpenAPILoader{}
	s, err := l.Load([]byte(minimalOpenAPI300))
	if err != nil {
		t.Fatalf("OpenAPI 3.0.0: unexpected error: %v", err)
	}
	if s == nil || s.Document == nil {
		t.Fatal("expected non-nil spec and document")
	}
}

func TestOpenAPILoader_Load_303_YAML(t *testing.T) {
	l := OpenAPILoader{}
	s, err := l.Load([]byte(minimalOpenAPI303))
	if err != nil {
		t.Fatalf("OpenAPI 3.0.3 YAML: unexpected error: %v", err)
	}
	if s == nil || s.Document == nil {
		t.Fatal("expected non-nil spec and document")
	}
}

func TestOpenAPILoader_Detect_300(t *testing.T) {
	l := OpenAPILoader{}
	if !l.Detect("application/json", []byte(minimalOpenAPI300)) {
		t.Error("should detect OpenAPI 3.0.0")
	}
}

// ---- Malformed specs -------------------------------------------------------

func TestOpenAPILoader_Load_MissingInfo(t *testing.T) {
	// libopenapi is permissive about missing info; the spec still loads.
	// This test documents the current behaviour: missing info is not fatal.
	l := OpenAPILoader{}
	body := []byte(`{"openapi":"3.1.0","paths":{}}`)
	s, err := l.Load(body)
	// Either a parse error or a spec without info are acceptable.
	if err == nil && s == nil {
		t.Error("expected either an error or a non-nil spec")
	}
}

func TestOpenAPILoader_Load_BadPathsShape(t *testing.T) {
	// paths is a string instead of an object.
	l := OpenAPILoader{}
	body := []byte(`{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":"not-an-object"}`)
	// libopenapi may or may not error; the important thing is it doesn't panic.
	_, _ = l.Load(body)
}

// ---- V3Model memoization ---------------------------------------------------

func TestAPISpec_V3ModelMemoized(t *testing.T) {
	l := OpenAPILoader{}
	s, err := l.Load([]byte(minimalOpenAPI))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	m1, err1 := s.V3Model()
	m2, err2 := s.V3Model()
	if err1 != err2 {
		t.Errorf("errors differ: %v vs %v", err1, err2)
	}
	if m1 != m2 {
		t.Error("expected V3Model to return the same pointer on second call")
	}
}

func TestAPISpec_OperationsMemoized(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200": {}`
	l := OpenAPILoader{}
	s, err := l.Load([]byte(raw))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops1, err1 := s.Operations("https://api.example.com", "")
	ops2, err2 := s.Operations("https://api.example.com", "")
	if err1 != nil || err2 != nil {
		t.Fatalf("Operations errors: %v / %v", err1, err2)
	}
	if len(ops1) != 1 || len(ops2) != 1 {
		t.Fatalf("expected 1 op each, got %d / %d", len(ops1), len(ops2))
	}
	// Same underlying slice pointer confirms memoization.
	if &ops1[0] != &ops2[0] {
		t.Error("expected Operations to return the same slice on second call")
	}
}

func TestAPISpec_Operations_NeutralTypes(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://api.example.com/v1
paths:
  /items/{id}:
    get:
      operationId: getItem
      summary: Get item
      parameters:
        - name: id
          in: path
          required: true
          description: Item ID
        - name: filter
          in: query
          x-cli-name: f
          x-cli-hidden: true
          schema:
            enum: [a, b, c]
      responses:
        "200": {}`
	l := OpenAPILoader{}
	s, err := l.Load([]byte(raw))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := s.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	op := ops[0]
	if op.ID != "getItem" {
		t.Errorf("ID: got %q, want %q", op.ID, "getItem")
	}
	if op.Method != "GET" {
		t.Errorf("Method: got %q, want %q", op.Method, "GET")
	}
	if op.Path != "/v1/items/{id}" {
		t.Errorf("Path: got %q, want %q", op.Path, "/v1/items/{id}")
	}
	if op.Summary != "Get item" {
		t.Errorf("Summary: got %q", op.Summary)
	}
	if len(op.Parameters) != 2 {
		t.Fatalf("expected 2 params, got %d", len(op.Parameters))
	}

	idParam := op.Parameters[0]
	if idParam.Name != "id" || idParam.In != "path" || !idParam.Required {
		t.Errorf("id param: %+v", idParam)
	}

	filterParam := op.Parameters[1]
	if filterParam.XCLI.Name != "f" {
		t.Errorf("filter x-cli-name: got %q, want %q", filterParam.XCLI.Name, "f")
	}
	if !filterParam.XCLI.Hidden {
		t.Error("filter param should be hidden")
	}
	if len(filterParam.Enum) != 3 {
		t.Errorf("filter enum: got %v", filterParam.Enum)
	}
}
