package spec

import (
	"reflect"
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
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

func TestOperationsExtractsEffectiveCredentialRequirements(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
security:
  - UserOAuth: [items:read]
components:
  securitySchemes:
    UserOAuth:
      type: oauth2
      flows:
        authorizationCode:
          authorizationUrl: https://auth.example.com/authorize
          tokenUrl: https://auth.example.com/token
          scopes:
            items:read: Read items
            items:write: Write items
    AdminOAuth:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/token
          scopes:
            admin:read: Read admin data
    PartnerKey:
      type: apiKey
      in: header
      name: X-Partner-Key
    BasicAuth:
      type: http
      scheme: basic
    BearerAuth:
      type: http
      scheme: bearer
    OIDC:
      type: openIdConnect
      openIdConnectUrl: https://auth.example.com/.well-known/openid-configuration
    ClientCert:
      type: mutualTLS
    urn:example:auth:TenantKey:
      type: apiKey
      in: header
      name: X-Tenant-Key
paths:
  /items:
    get:
      operationId: listItems
      responses:
        "200":
          description: OK
  /admin:
    get:
      operationId: adminUsers
      security:
        - AdminOAuth: [admin:read]
      responses:
        "200":
          description: OK
  /public:
    get:
      operationId: status
      security: []
      responses:
        "200":
          description: OK
  /optional:
    get:
      operationId: optionalPartner
      security:
        - {}
        - PartnerKey: []
      responses:
        "200":
          description: OK
  /either:
    get:
      operationId: partnerReport
      security:
        - UserOAuth: [items:read]
        - PartnerKey: []
      responses:
        "200":
          description: OK
  /signed:
    get:
      operationId: signedReport
      security:
        - UserOAuth: [items:read]
          PartnerKey: []
      responses:
        "200":
          description: OK
  /roles:
    get:
      operationId: roleReport
      security:
        - BasicAuth: [admin, reports]
      responses:
        "200":
          description: OK
  /uri:
    get:
      operationId: tenantReport
      security:
        - urn:example:auth:TenantKey: [tenant:read]
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}

	inherited := operationByID(t, ops, "listItems")
	requireCredential(t, inherited, [][]CredentialRequirement{{
		{ID: "UserOAuth", Ref: "#/components/securitySchemes/UserOAuth", Kind: "oauth2", Needs: []string{"items:read"}, Source: "openapi"},
	}})

	override := operationByID(t, ops, "adminUsers")
	requireCredential(t, override, [][]CredentialRequirement{{
		{ID: "AdminOAuth", Ref: "#/components/securitySchemes/AdminOAuth", Kind: "oauth2", Needs: []string{"admin:read"}, Source: "openapi"},
	}})

	public := operationByID(t, ops, "status")
	if !public.NoAuth {
		t.Fatal("status NoAuth = false, want true")
	}
	if len(public.CredentialAlternatives) != 0 {
		t.Fatalf("status alternatives = %#v, want none", public.CredentialAlternatives)
	}

	optional := operationByID(t, ops, "optionalPartner")
	if !optional.OptionalAuth {
		t.Fatal("optionalPartner OptionalAuth = false, want true")
	}
	requireCredential(t, optional, [][]CredentialRequirement{{
		{ID: "PartnerKey", Ref: "#/components/securitySchemes/PartnerKey", Kind: "api-key", Source: "openapi"},
	}})

	either := operationByID(t, ops, "partnerReport")
	requireCredential(t, either, [][]CredentialRequirement{
		{{ID: "UserOAuth", Ref: "#/components/securitySchemes/UserOAuth", Kind: "oauth2", Needs: []string{"items:read"}, Source: "openapi"}},
		{{ID: "PartnerKey", Ref: "#/components/securitySchemes/PartnerKey", Kind: "api-key", Source: "openapi"}},
	})

	combined := operationByID(t, ops, "signedReport")
	requireCredential(t, combined, [][]CredentialRequirement{{
		{ID: "UserOAuth", Ref: "#/components/securitySchemes/UserOAuth", Kind: "oauth2", Needs: []string{"items:read"}, Source: "openapi"},
		{ID: "PartnerKey", Ref: "#/components/securitySchemes/PartnerKey", Kind: "api-key", Source: "openapi"},
	}})

	roles := operationByID(t, ops, "roleReport")
	requireCredential(t, roles, [][]CredentialRequirement{{
		{ID: "BasicAuth", Ref: "#/components/securitySchemes/BasicAuth", Kind: "http-basic", Needs: []string{"admin", "reports"}, Source: "openapi"},
	}})

	uri := operationByID(t, ops, "tenantReport")
	requireCredential(t, uri, [][]CredentialRequirement{{
		{ID: "urn:example:auth:TenantKey", Ref: "urn:example:auth:TenantKey", Kind: "api-key", Needs: []string{"tenant:read"}, Source: "openapi", External: true},
	}})
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

	ops, err := loaded.Operations(OperationOptions{
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com/root"})
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com/root", OperationBase: "/"})
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

func TestOperationsRelativeServerURLWithTrailingSlashBase(t *testing.T) {
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com/root/"})
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

func TestOperationsAbsoluteServerOutsideBasePathUsesRelativeEscape(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://api.example.com/v2
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com/root"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/../v2/items" {
		t.Fatalf("operation path = %q, want /../v2/items", got)
	}
}

func TestOperationsRootRelativeServerOutsideBasePathUsesRelativeEscape(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: /v1
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com/root"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/../v1/items" {
		t.Fatalf("operation path = %q, want /../v1/items", got)
	}
}

func operationByID(t *testing.T, ops []Operation, id string) Operation {
	t.Helper()
	for _, op := range ops {
		if op.ID == id {
			return op
		}
	}
	t.Fatalf("operation %q not found in %#v", id, ops)
	return Operation{}
}

func requireCredential(t *testing.T, op Operation, want [][]CredentialRequirement) {
	t.Helper()
	var got [][]CredentialRequirement
	for _, alternative := range op.CredentialAlternatives {
		got = append(got, []CredentialRequirement(alternative))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s credential alternatives:\ngot  %#v\nwant %#v", op.ID, got, want)
	}
}

func TestOperationsAbsoluteNonMatchingServerFallsBackToAPIBase(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
servers:
  - url: https://other.example.com/v2
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com/root"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if got := ops[0].Path; got != "/items" {
		t.Fatalf("operation path = %q, want /items", got)
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

	_, err = loaded.Operations(OperationOptions{
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

	_, err = loaded.Operations(OperationOptions{
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

	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
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
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
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
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
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

func TestOperationSetToleratesCircularSchemas(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Circular
  version: "1.0.0"
paths:
  /nodes:
    get:
      operationId: listNodes
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
      type: object
      properties:
        child:
          $ref: "#/components/schemas/Node"`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	set, err := loaded.OperationSet(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operation set: %v", err)
	}
	if len(set.Operations) != 1 || set.Operations[0].ID != "listNodes" {
		t.Fatalf("operations = %#v", set.Operations)
	}
}

func TestOpenAPI31PatchVersionLoads(t *testing.T) {
	raw := `openapi: "3.1.1"
info:
  title: Patch Version
  version: "1.0.0"
paths: {}`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded spec")
	}
}

func TestOpenAPIWebhooksWithoutPathsProducesNoOperations(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Webhooks Only
  version: "1.0.0"
webhooks:
  itemCreated:
    post:
      operationId: itemCreated
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("operations = %#v, want none", ops)
	}
}

func TestOpenAPIEmptyPathItemsAreIgnored(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Empty Path Items
  version: "1.0.0"
paths:
  /empty: {}`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 0 {
		t.Fatalf("operations = %#v, want none", ops)
	}
}

func TestOpenAPITraceOperationsAreExtracted(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Trace Operation
  version: "1.0.0"
paths:
  /diagnostics:
    trace:
      operationId: traceDiagnostics
      responses:
        "204":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if ops[0].Method != "TRACE" || ops[0].ID != "traceDiagnostics" {
		t.Fatalf("operation = %#v, want TRACE traceDiagnostics", ops[0])
	}
}

func TestOpenAPIIgnoresReservedHeaderParameters(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Reserved Headers
  version: "1.0.0"
paths:
  /items:
    get:
      operationId: listItems
      parameters:
        - name: Accept
          in: header
          required: true
          schema:
            type: string
        - name: Content-Type
          in: header
          required: true
          schema:
            type: string
        - name: Authorization
          in: header
          required: true
          schema:
            type: string
        - name: X-Trace
          in: header
          schema:
            type: string
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if len(ops[0].Parameters) != 1 || ops[0].Parameters[0].Name != "X-Trace" {
		t.Fatalf("parameters = %#v, want only X-Trace", ops[0].Parameters)
	}
}

func TestOpenAPIComponentParameterRefsAndOperationOverride(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Component Parameters
  version: "1.0.0"
paths:
  /items:
    parameters:
      - $ref: "#/components/parameters/Limit"
    get:
      operationId: listItems
      parameters:
        - name: limit
          in: query
          required: true
          description: Operation-level limit
          schema:
            type: integer
      responses:
        "200":
          description: OK
components:
  parameters:
    Limit:
      name: limit
      in: query
      description: Path-level limit
      schema:
        type: string`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("len(ops) = %d, want 1", len(ops))
	}
	if len(ops[0].Parameters) != 1 {
		t.Fatalf("parameters = %#v, want one overridden limit param", ops[0].Parameters)
	}
	got := ops[0].Parameters[0]
	if got.Name != "limit" || got.In != "query" || !got.Required || got.Type != "integer" || got.Desc != "Operation-level limit" {
		t.Fatalf("limit parameter = %#v, want operation-level integer required parameter", got)
	}
}

func TestOpenAPICallbacksAndLinksDoNotCreateExtraOperations(t *testing.T) {
	raw := `openapi: "3.1.0"
info:
  title: Callback Metadata
  version: "1.0.0"
paths:
  /subscriptions:
    post:
      operationId: createSubscription
      callbacks:
        onEvent:
          "{$request.body#/callbackUrl}":
            post:
              operationId: callbackEvent
              responses:
                "200":
                  description: OK
      responses:
        "201":
          description: Created
          links:
            getSubscription:
              operationId: getSubscription
              parameters:
                id: "$response.body#/id"
  /subscriptions/{id}:
    get:
      operationId: getSubscription
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK`
	loaded, err := load("application/yaml", []byte(raw), DefaultLoaders())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ops, err := loaded.Operations(OperationOptions{BaseURL: "https://api.example.com"})
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("operations = %#v, want only concrete path operations", ops)
	}
	ids := map[string]bool{}
	for _, op := range ops {
		ids[op.ID] = true
	}
	if ids["callbackEvent"] {
		t.Fatalf("callback operation should not be generated as a request command: %#v", ops)
	}
	if !ids["createSubscription"] || !ids["getSubscription"] {
		t.Fatalf("operations = %#v, want createSubscription and getSubscription", ops)
	}
}
