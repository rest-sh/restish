package spec

import (
	"strings"
	"testing"
)

func TestOperationsUsesServerVariableDefaultsWithoutEnumExpansion(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://{env}.example.com/{version}
    variables:
      env:
        default: api
        enum: [api, staging, dev, qa]
      version:
        default: v1
        enum: [v1, v2, v3, v4]
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/v1/items" {
		t.Fatalf("operation path = %q, want /v1/items", got)
	}
}

func TestOperationsUsesConfiguredServerVariables(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://{env}.example.com/{version}
    variables:
      env:
        default: api
        enum: [api, staging]
      version:
        default: v1
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ops, err := loaded.OperationsWithOptions(OperationOptions{
		BaseURL: "https://staging.example.com",
		ServerVariables: map[string]string{
			"env":     "staging",
			"version": "v2",
		},
	})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/v2/items" {
		t.Fatalf("operation path = %q, want /v2/items", got)
	}
}

func TestOperationsUsesEffectivePathAndOperationServers(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: /doc
paths:
  /items:
    servers:
      - url: /path
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
  /widgets:
    servers:
      - url: /path
    get:
      operationId: listWidgets
      servers:
        - url: /op
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("len(ops) = %d, want 2", len(ops))
	}
	paths := map[string]string{}
	for _, op := range ops {
		paths[op.ID] = op.Path
	}
	if got := paths["listItems"]; got != "/path/items" {
		t.Fatalf("listItems path = %q, want /path/items", got)
	}
	if got := paths["listWidgets"]; got != "/op/widgets" {
		t.Fatalf("listWidgets path = %q, want /op/widgets", got)
	}
}

func TestOperationsResolvesRelativeServerURLAgainstAPIBase(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: "{version}"
    variables:
      version:
        default: v2
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ops, err := loaded.Operations("https://api.example.com/root", "")
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/v2/items" {
		t.Fatalf("operation path = %q, want /v2/items", got)
	}
}

func TestOperationsOperationBaseIgnoresRelativeServerURL(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: v2
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ops, err := loaded.Operations("https://api.example.com/root", "/")
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/items" {
		t.Fatalf("operation path = %q, want /items when operation_base is set", got)
	}
}

func TestOperationsRejectsUnknownConfiguredServerVariable(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://api.example.com/{version}
    variables:
      version:
        default: v1
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	_, err = loaded.OperationsWithOptions(OperationOptions{
		BaseURL:         "https://api.example.com",
		ServerVariables: map[string]string{"env": "staging"},
	})
	if err == nil || !strings.Contains(err.Error(), `server variable "env"`) {
		t.Fatalf("expected unknown server variable error, got %v", err)
	}
}

func TestOperationsRejectsConfiguredServerVariableEnumMismatch(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://{env}.example.com
    variables:
      env:
        default: api
        enum: [api, staging]
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	_, err = loaded.OperationsWithOptions(OperationOptions{
		BaseURL:         "https://api.example.com",
		ServerVariables: map[string]string{"env": "prod"},
	})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected enum mismatch error, got %v", err)
	}
}

func TestOperationsServerVariableResolutionDoesNotBuildCartesianProduct(t *testing.T) {
	var b strings.Builder
	b.WriteString(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://api.example.com/{a}/{b}/{c}/{d}
    variables:
`)
	for _, name := range []string{"a", "b", "c", "d"} {
		b.WriteString("      ")
		b.WriteString(name)
		b.WriteString(":\n        default: x\n        enum:\n")
		for i := 0; i < 100; i++ {
			b.WriteString("          - v")
			b.WriteString(string(rune('a' + i%26)))
			b.WriteString("\n")
		}
	}
	b.WriteString(`paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK`)
	loaded, err := load("application/yaml", []byte(b.String()), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/x/x/x/x/items" {
		t.Fatalf("operation path = %q, want /x/x/x/x/items", got)
	}
}

func TestOpenAPIOperationMissingResponsesDoesNotPanic(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Missing Responses
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if ops[0].ID != "listItems" {
		t.Fatalf("operation ID = %q", ops[0].ID)
	}
}

func TestOpenAPINullDefaultAndCircularAllOfDoNotPanic(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Regression Fixture
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: maybe
          in: query
          schema:
            type: [string, "null"]
            nullable: true
            default: null
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Node"
components:
  schemas:
    Node:
      allOf:
        - type: object
          properties:
            id:
              type: string
        - $ref: "#/components/schemas/Node"`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations("https://api.example.com", "")
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if len(ops[0].Parameters) != 1 {
		t.Fatalf("parameters = %#v", ops[0].Parameters)
	}
}
