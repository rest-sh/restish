package spec

import (
	"os"
	"path/filepath"
	"strings"
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

func TestOpenAPILoader_Detect_TextPlainOpenAPI(t *testing.T) {
	l := OpenAPILoader{}
	if !l.Detect("text/plain", []byte(minimalOpenAPIYAML)) {
		t.Error("should detect OpenAPI YAML served as text/plain")
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

func TestOpenAPILoader_Load_IgnoresDescriptionRefObjects(t *testing.T) {
	raw := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
  description:
    $ref: markdown/info.md
tags:
  - name: widgets
    description:
      $ref: markdown/widgets.md
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: q
          in: query
          description:
            $ref: markdown/query.md
          schema:
            type: string
      responses:
        "200":
          description:
            $ref: markdown/ok.md`)
	l := OpenAPILoader{}
	s, err := l.Load(raw)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	info, err := s.Info()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Description != "" {
		t.Fatalf("description = %q, want ignored", info.Description)
	}
	ops, err := s.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if len(ops) != 1 || ops[0].ID != "listItems" {
		t.Fatalf("operations = %#v", ops)
	}
	if len(ops[0].Parameters) != 1 || ops[0].Parameters[0].Desc != "" {
		t.Fatalf("parameter description = %#v, want ignored", ops[0].Parameters)
	}
}

func TestOpenAPILoader_Load_DescriptionRefObjectDoesNotFetchExternalDoc(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "openapi.yaml")
	raw := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
tags:
  - name: widgets
    description:
      $ref: missing.md
paths: {}`)
	if err := os.WriteFile(specPath, raw, 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	l := OpenAPILoader{}
	if _, err := l.LoadWithOptions(raw, LoadOptions{LocalPath: specPath}); err != nil {
		t.Fatalf("load should not fetch description ref: %v", err)
	}
}

func TestOpenAPILoader_Load_ExternalDocDescriptionRefObjectDoesNotFetchMarkdown(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "openapi.yaml")
	componentsPath := filepath.Join(dir, "components.yaml")
	root := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: components.yaml#/components/schemas/Item`)
	components := []byte(`components:
  schemas:
    Item:
      type: object
      description:
        $ref: missing.md
      properties:
        id:
          type: string`)
	if err := os.WriteFile(rootPath, root, 0o600); err != nil {
		t.Fatalf("write root: %v", err)
	}
	if err := os.WriteFile(componentsPath, components, 0o600); err != nil {
		t.Fatalf("write components: %v", err)
	}
	l := OpenAPILoader{}
	s, err := l.LoadWithOptions(root, LoadOptions{LocalPath: rootPath})
	if err != nil {
		t.Fatalf("load should not fetch external doc description ref: %v", err)
	}
	ops, err := s.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("Operations: %v", err)
	}
	if len(ops) != 1 || ops[0].ID != "listItems" {
		t.Fatalf("operation response help = %#v", ops)
	}
}

func TestOpenAPILoader_Load_ExternalDocRootDescriptionEntryPreservesRealRef(t *testing.T) {
	dir := t.TempDir()
	rootPath := filepath.Join(dir, "openapi.yaml")
	schemasPath := filepath.Join(dir, "schemas.yaml")
	commonPath := filepath.Join(dir, "common.yaml")
	root := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: schemas.yaml#/description`)
	schemas := []byte(`description:
  $ref: common.yaml#/Description`)
	common := []byte(`Description:
  type: string`)
	if err := os.WriteFile(rootPath, root, 0o600); err != nil {
		t.Fatalf("write root: %v", err)
	}
	if err := os.WriteFile(schemasPath, schemas, 0o600); err != nil {
		t.Fatalf("write schemas: %v", err)
	}
	if err := os.WriteFile(commonPath, common, 0o600); err != nil {
		t.Fatalf("write common: %v", err)
	}
	resolved, err := resolveOpenAPIExternalRefs(root, LoadOptions{LocalPath: rootPath})
	if err != nil {
		t.Fatalf("resolve refs: %v", err)
	}
	if !strings.Contains(string(resolved), "type: string") {
		t.Fatalf("external root entry named description was not resolved:\n%s", resolved)
	}
}

func TestSanitizeOpenAPIDescriptionRefsPreservesSchemaProperties(t *testing.T) {
	raw := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  schemas:
    StringValue:
      type: string
    Item:
      type: object
      properties:
        description:
          $ref: "#/components/schemas/StringValue"`)
	sanitized, err := sanitizeOpenAPIDescriptionRefs(raw)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if string(sanitized) != string(raw) {
		t.Fatalf("schema property named description should be preserved:\n%s", sanitized)
	}
}

func TestSanitizeOpenAPIDescriptionRefsIgnoresRootSchemaDescriptionRef(t *testing.T) {
	raw := []byte(`type: object
description:
  $ref: markdown/schema.md
properties:
  id:
    type: string`)
	sanitized, err := sanitizeOpenAPIDescriptionRefs(raw)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if strings.Contains(string(sanitized), "markdown/schema.md") {
		t.Fatalf("root schema description ref should be ignored:\n%s", sanitized)
	}
}

func TestSanitizeOpenAPIDescriptionRefsPreservesArbitraryNamedMaps(t *testing.T) {
	raw := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
webhooks:
  description:
    $ref: "#/components/pathItems/Hook"
components:
  pathItems:
    Hook:
      post:
        responses:
          "200":
            description: OK
  schemas:
    Item:
      type: object
      dependentSchemas:
        description:
          $ref: "#/components/schemas/StringValue"
    StringValue:
      type: string`)
	sanitized, err := sanitizeOpenAPIDescriptionRefs(raw)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if string(sanitized) != string(raw) {
		t.Fatalf("arbitrary named map entries should be preserved:\n%s", sanitized)
	}
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
	ops1, err1 := s.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	ops2, err2 := s.Operations(OperationOptions{BaseURL: "https://api.example.com"})
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
	ops, err := s.Operations(OperationOptions{BaseURL: "https://api.example.com"})
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
