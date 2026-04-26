package spec

import (
	"testing"
)

// loadDoc is a test helper that parses a raw OpenAPI document into an APISpec.
func loadDoc(t *testing.T, raw string) *APISpec {
	t.Helper()
	l := OpenAPILoader{}
	s, err := l.Load([]byte(raw))
	if err != nil {
		t.Fatalf("loadDoc: %v", err)
	}
	return s
}

// ---- ReadXCLIConfig --------------------------------------------------------

func TestReadXCLIConfig_Present(t *testing.T) {
	raw := []byte(`
x-cli-config:
  profiles:
    default:
      auth:
        type: bearer
        params:
          token: ""
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}`)
	spec := &APISpec{Raw: raw}
	cfg, err := ReadXCLIConfig(spec)
	if err != nil {
		t.Fatalf("ReadXCLIConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Profiles["default"] == nil {
		t.Fatal("expected default profile")
	}
	if cfg.Profiles["default"].Auth == nil {
		t.Fatal("expected auth in default profile")
	}
	if cfg.Profiles["default"].Auth.Type != "bearer" {
		t.Errorf("auth type: got %q, want %q", cfg.Profiles["default"].Auth.Type, "bearer")
	}
}

func TestReadXCLIConfig_LegacyPromptShapeNormalizesToDefaultProfile(t *testing.T) {
	raw := []byte(`
x-cli-config:
  security: default
  headers:
    Accept: application/json
  prompt:
    client_id:
      description: Client identifier
      example: abc123
  params:
    audience: https://example.com/{client_id}
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}`)
	spec := &APISpec{Raw: raw}
	cfg, err := ReadXCLIConfig(spec)
	if err != nil {
		t.Fatalf("ReadXCLIConfig: %v", err)
	}
	normalized := cfg.Normalize()
	profile := normalized.Profiles["default"]
	if profile == nil {
		t.Fatal("expected legacy config to normalize to default profile")
	}
	if profile.Security != "default" {
		t.Fatalf("Security = %q, want default", profile.Security)
	}
	if got := profile.Headers; len(got) != 1 || got[0] != "Accept: application/json" {
		t.Fatalf("Headers = %#v", got)
	}
	if profile.Prompt["client_id"].Description != "Client identifier" {
		t.Fatalf("Prompt = %#v", profile.Prompt)
	}
	if profile.Params["audience"] != "https://example.com/{client_id}" {
		t.Fatalf("Params = %#v", profile.Params)
	}
}

func TestReadXCLIConfig_Absent(t *testing.T) {
	raw := []byte(`openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}`)
	spec := &APISpec{Raw: raw}
	cfg, err := ReadXCLIConfig(spec)
	if err != nil {
		t.Fatalf("ReadXCLIConfig: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config when extension absent")
	}
}

// ---- SchemeToXCLIAuth ------------------------------------------------------

func TestSchemeToXCLIAuth_HTTPBasic(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    basic:
      type: http
      scheme: basic`
	doc := loadDoc(t, raw)
	model, err := doc.V3Model()
	if err != nil || model == nil {
		t.Fatalf("BuildV3Model: %v", err)
	}
	scheme := model.Model.Components.SecuritySchemes.GetOrZero("basic")
	if scheme == nil {
		t.Fatal("scheme not found")
	}
	auth := SchemeToXCLIAuth(scheme, nil)
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
	if auth.Type != "http-basic" {
		t.Errorf("Type: got %q, want %q", auth.Type, "http-basic")
	}
	if _, ok := auth.Params["username"]; !ok {
		t.Error("expected username param")
	}
	if _, ok := auth.Params["password"]; !ok {
		t.Error("expected password param")
	}
}

func TestSchemeToXCLIAuth_OAuth2AuthCode(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    oauth:
      type: oauth2
      flows:
        authorizationCode:
          authorizationUrl: https://auth.example.com/oauth/authorize
          tokenUrl: https://auth.example.com/oauth/token
          scopes: {}`
	doc := loadDoc(t, raw)
	model, err := doc.V3Model()
	if err != nil || model == nil {
		t.Fatalf("BuildV3Model: %v", err)
	}
	scheme := model.Model.Components.SecuritySchemes.GetOrZero("oauth")
	if scheme == nil {
		t.Fatal("scheme not found")
	}
	auth := SchemeToXCLIAuth(scheme, nil)
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
	if auth.Type != "oauth-authorization-code" {
		t.Errorf("Type: got %q, want %q", auth.Type, "oauth-authorization-code")
	}
}

func TestSchemeToXCLIAuth_OAuth2ClientCredentials(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    creds:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/oauth/token
          scopes: {}`
	doc := loadDoc(t, raw)
	model, err := doc.V3Model()
	if err != nil || model == nil {
		t.Fatalf("BuildV3Model: %v", err)
	}
	scheme := model.Model.Components.SecuritySchemes.GetOrZero("creds")
	if scheme == nil {
		t.Fatal("scheme not found")
	}
	auth := SchemeToXCLIAuth(scheme, nil)
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
	if auth.Type != "oauth-client-credentials" {
		t.Errorf("Type: got %q, want %q", auth.Type, "oauth-client-credentials")
	}
}

func TestSchemeToXCLIAuth_UnsupportedType(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    apikey:
      type: apiKey
      in: header
      name: X-API-Key`
	doc := loadDoc(t, raw)
	model, err := doc.V3Model()
	if err != nil || model == nil {
		t.Fatalf("BuildV3Model: %v", err)
	}
	scheme := model.Model.Components.SecuritySchemes.GetOrZero("apikey")
	if scheme == nil {
		t.Fatal("scheme not found")
	}
	auth := SchemeToXCLIAuth(scheme, nil)
	if auth != nil {
		t.Errorf("expected nil for unsupported scheme type, got %v", auth)
	}
}

func TestSchemeToXCLIAuth_ParamOverrides(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    basic:
      type: http
      scheme: basic`
	doc := loadDoc(t, raw)
	model, _ := doc.V3Model()
	scheme := model.Model.Components.SecuritySchemes.GetOrZero("basic")
	auth := SchemeToXCLIAuth(scheme, map[string]string{"username": "alice"})
	if auth.Params["username"] != "alice" {
		t.Errorf("username param override: got %q, want %q", auth.Params["username"], "alice")
	}
}

// ---- ExpandParams ----------------------------------------------------------

func TestExpandParams(t *testing.T) {
	tests := []struct {
		s, want string
		params  map[string]string
	}{
		{"Authorization: Bearer {token}", "Authorization: Bearer abc123", map[string]string{"token": "abc123"}},
		{"no placeholders", "no placeholders", map[string]string{"token": "x"}},
		{"{a}/{b}", "1/2", map[string]string{"a": "1", "b": "2"}},
		{"{unknown}", "{unknown}", map[string]string{"other": "x"}},
		{"", "", nil},
	}
	for _, tc := range tests {
		got := ExpandParams(tc.s, tc.params)
		if got != tc.want {
			t.Errorf("ExpandParams(%q, %v) = %q, want %q", tc.s, tc.params, got, tc.want)
		}
	}
}

// ---- FallbackXCLIConfig ----------------------------------------------------

func TestFallbackXCLIConfig_BasicAuth(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
security:
  - basic: []
components:
  securitySchemes:
    basic:
      type: http
      scheme: basic`
	doc := loadDoc(t, raw)
	cfg := FallbackXCLIConfig(doc)
	if cfg == nil {
		t.Fatal("expected non-nil fallback config")
	}
	p := cfg.Profiles["default"]
	if p == nil {
		t.Fatal("expected default profile")
	}
	if p.Auth == nil || p.Auth.Type != "http-basic" {
		t.Errorf("expected http-basic auth, got %v", p.Auth)
	}
}

func TestFallbackXCLIConfig_NoSupportedScheme(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    apikey:
      type: apiKey
      in: header
      name: X-API-Key`
	doc := loadDoc(t, raw)
	cfg := FallbackXCLIConfig(doc)
	// apiKey is not supported, so should return nil.
	if cfg != nil {
		t.Errorf("expected nil for unsupported scheme, got %v", cfg)
	}
}

// ---- Resolve ---------------------------------------------------------------

func TestResolve_SecurityToAuth(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    basic:
      type: http
      scheme: basic`
	spec := loadDoc(t, raw)

	xcli := &XCLIConfig{
		Profiles: map[string]*XCLIProfile{
			"default": {Security: "basic"},
		},
	}
	resolved := xcli.Resolve(spec)
	p := resolved.Profiles["default"]
	if p == nil {
		t.Fatal("expected default profile in resolved config")
	}
	if p.Auth == nil || p.Auth.Type != "http-basic" {
		t.Errorf("expected http-basic auth, got %v", p.Auth)
	}
}

func TestResolve_HeaderExpansion(t *testing.T) {
	xcli := &XCLIConfig{
		Profiles: map[string]*XCLIProfile{
			"default": {
				Headers: []string{"Authorization: Bearer {token}"},
				Params:  map[string]string{"token": "mytoken"},
			},
		},
	}
	resolved := xcli.Resolve(nil)
	p := resolved.Profiles["default"]
	if len(p.Headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(p.Headers))
	}
	if p.Headers[0] != "Authorization: Bearer mytoken" {
		t.Errorf("got %q, want %q", p.Headers[0], "Authorization: Bearer mytoken")
	}
}

func TestResolve_PromptedHeaderExpansionIsSinglePass(t *testing.T) {
	xcli := &XCLIConfig{
		Profiles: map[string]*XCLIProfile{
			"default": {
				Headers: []string{"X-Token: {token}"},
				Params:  map[string]string{"token": "{org}", "org": "acme"},
				PromptedParams: map[string]bool{
					"token": true,
				},
			},
		},
	}
	resolved := xcli.Resolve(nil)
	got := resolved.Profiles["default"].Headers[0]
	if got != "X-Token: {org}" {
		t.Fatalf("header = %q, want prompted value preserved", got)
	}
}

func TestResolve_ExplicitAuthTakesPrecedence(t *testing.T) {
	raw := `
openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths: {}
components:
  securitySchemes:
    basic:
      type: http
      scheme: basic`
	spec := loadDoc(t, raw)

	explicit := &XCLIAuth{Type: "bearer", Params: map[string]string{"token": ""}}
	xcli := &XCLIConfig{
		Profiles: map[string]*XCLIProfile{
			"default": {Security: "basic", Auth: explicit},
		},
	}
	resolved := xcli.Resolve(spec)
	p := resolved.Profiles["default"]
	if p.Auth.Type != "bearer" {
		t.Errorf("expected explicit bearer auth to win, got %q", p.Auth.Type)
	}
}
