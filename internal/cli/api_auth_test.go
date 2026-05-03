package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
)

func TestAPIAuthListAddRemove(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {}
      }
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "add", "myapi", "PartnerKey"}); err != nil {
		t.Fatalf("api auth add: %v", err)
	}
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if written.APIs["myapi"].Profiles["default"].Credentials["PartnerKey"] == nil {
		t.Fatal("expected PartnerKey credential")
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "list", "myapi"}); err != nil {
		t.Fatalf("api auth list: %v", err)
	}
	if !strings.Contains(out.String(), "PartnerKey") {
		t.Fatalf("expected PartnerKey in list output, got %q", out.String())
	}

	if err := c.Run([]string{"restish", "api", "auth", "remove", "myapi", "PartnerKey"}); err != nil {
		t.Fatalf("api auth remove: %v", err)
	}
	written, err = config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if written.APIs["myapi"].Profiles["default"].Credentials != nil {
		t.Fatalf("expected credentials removed, got %#v", written.APIs["myapi"].Profiles["default"].Credentials)
	}
}

func TestAPIAuthInspectCredentialAPIKey(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {
              "auth": {
                "type": "api-key",
                "params": {"in": "header", "name": "X-Partner-Key", "value": "secret"}
              }
            }
          }
        }
      }
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--rsh-credential", "PartnerKey"}); err != nil {
		t.Fatalf("api auth inspect: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "secret") || !strings.Contains(got, "X-Partner-Key: <redacted>") {
		t.Fatalf("expected redacted API key inspection, got %q", got)
	}
}

func TestAPIAuthInspectURLSuggestsV2Form(t *testing.T) {
	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "api", "auth", "inspect", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected URL argument error")
	}
	if !strings.Contains(err.Error(), "v2 form: restish api auth inspect <api-name> --raw-header Authorization") {
		t.Fatalf("expected v2 form hint, got: %v", err)
	}
}

func TestAPIAuthInspectSingleCredentialByDefault(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {
              "auth": {
                "type": "api-key",
                "params": {"in": "header", "name": "X-Partner-Key", "value": "secret"}
              }
            }
          }
        }
      }
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi"}); err != nil {
		t.Fatalf("api auth inspect: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Credential: PartnerKey") {
		t.Fatalf("expected selected credential in output, got %q", got)
	}
	if strings.Contains(got, "secret") || !strings.Contains(got, "X-Partner-Key: <redacted>") {
		t.Fatalf("expected redacted API key inspection, got %q", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--raw-header", "X-Partner-Key"}); err != nil {
		t.Fatalf("api auth inspect raw header: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "secret" {
		t.Fatalf("raw header = %q, want secret", got)
	}
}

func TestAPIAuthInspectMultipleCredentialsRequiresSelection(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {
              "auth": {
                "type": "api-key",
                "params": {"in": "header", "name": "X-Partner-Key", "value": "secret"}
              }
            },
            "UserBearer": {
              "auth": {
                "type": "bearer",
                "params": {"token": "user-token"}
              }
            }
          }
        }
      }
    }
  }
}`)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi"})
	if err == nil {
		t.Fatal("expected multiple credentials error")
	}
	got := err.Error()
	for _, want := range []string{"multiple configured credentials", "PartnerKey, UserBearer", "--rsh-credential"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestAPIAuthListUsesCachedOperationMetadata(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "UserOAuth": {"type": "oauth2", "flows": {"authorizationCode": {"authorizationUrl": "https://auth.example.com/authorize", "tokenUrl": "https://auth.example.com/token", "scopes": {"items:read": "Read items"}}}},
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"},
      "OldKey": {"type": "apiKey", "in": "header", "name": "X-Old-Key", "deprecated": true},
      "MutualTLS": {"type": "mutualTLS"},
      "urn:example:auth:TenantKey": {"type": "apiKey", "in": "header", "name": "X-Tenant-Key"}
    }
  },
  "security": [{"UserOAuth": ["items:read"]}],
  "paths": {
    "/items": {"get": {"operationId": "listItems", "responses": {"200": {"description": "OK"}}}},
    "/partner": {"get": {"operationId": "partnerReport", "security": [{"PartnerKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/old": {"get": {"operationId": "oldReport", "security": [{"OldKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/mtls": {"get": {"operationId": "mtlsReport", "security": [{"MutualTLS": []}], "responses": {"200": {"description": "OK"}}}},
    "/tenant": {"get": {"operationId": "tenantReport", "security": [{"urn:example:auth:TenantKey": []}], "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	baseURL := readBaseURLFromConfig(t, env.cfgFile)
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {
			BaseURL: baseURL,
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					Credentials: map[string]*config.CredentialConfig{
						"UserOAuth": {
							Auth:      &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "user-token"}},
							Satisfies: []string{"items:read"},
						},
					},
				},
			},
		},
	}})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(env.cfgFile, 0o600); err != nil {
		t.Fatal(err)
	}

	c, out := env.newCaptureCLI()
	loader := &countingLoader{}
	c.AddLoader(loader)
	if err := c.Run([]string{"restish", "api", "auth", "list", "tapi"}); err != nil {
		t.Fatalf("api auth list: %v", err)
	}
	if got := loader.detects.Load(); got != 0 {
		t.Fatalf("api auth list loaded spec via loader %d times; want cached operation metadata only", got)
	}
	got := out.String()
	for _, want := range []string{
		"Callable secured operations: 1/5",
		"UserOAuth: configured, needs items:read, satisfies items:read",
		"PartnerKey: missing",
		"OldKey: missing, deprecated",
		"MutualTLS: missing, unsupported mtls",
		"urn:example:auth:TenantKey: missing, URI-backed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("list output missing %q:\n%s", want, got)
		}
	}
}

func TestAPIAuthAddDerivesAuthAndPromptsFromCachedSpec(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"}
    }
  },
  "paths": {
    "/partner": {"get": {"operationId": "partnerReport", "security": [{"PartnerKey": []}], "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	baseURL := readBaseURLFromConfig(t, env.cfgFile)
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {
			BaseURL: baseURL,
			Profiles: map[string]*config.ProfileConfig{
				"default": {},
			},
		},
	}})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(env.cfgFile, 0o600); err != nil {
		t.Fatal(err)
	}

	c := env.newCLI()
	c.Hooks().PassReader = strings.NewReader("partner-secret\n")
	if err := c.Run([]string{"restish", "api", "auth", "add", "tapi", "PartnerKey"}); err != nil {
		t.Fatalf("api auth add: %v", err)
	}
	written, err := config.Load(env.cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	credential := written.APIs["tapi"].Profiles["default"].Credentials["PartnerKey"]
	if credential == nil || credential.Auth == nil {
		t.Fatalf("credential = %#v", credential)
	}
	if credential.Auth.Type != "api-key" || credential.Auth.Params["name"] != "X-Partner-Key" || credential.Auth.Params["value"] != "partner-secret" {
		t.Fatalf("auth = %#v", credential.Auth)
	}
}

func TestAPIAuthInspectOperationCombinedCredentials(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "UserKey": {"type": "apiKey", "in": "header", "name": "X-User-Key"},
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"}
    }
  },
  "paths": {
    "/signed": {"get": {"operationId": "signedReport", "security": [{"UserKey": [], "PartnerKey": []}], "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	baseURL := readBaseURLFromConfig(t, env.cfgFile)
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {
			BaseURL: baseURL,
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					Credentials: map[string]*config.CredentialConfig{
						"UserKey": {
							Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-User-Key", "value": "user-secret"}},
						},
						"PartnerKey": {
							Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Partner-Key", "value": "partner-secret"}},
						},
					},
				},
			},
		},
	}})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(env.cfgFile, 0o600); err != nil {
		t.Fatal(err)
	}

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "tapi", "--rsh-operation", "signed-report"}); err != nil {
		t.Fatalf("api auth inspect operation: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Operation: signedReport",
		"Credentials: UserKey, PartnerKey",
		"X-User-Key: <redacted>",
		"X-Partner-Key: <redacted>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "user-secret") || strings.Contains(got, "partner-secret") {
		t.Fatalf("inspect output leaked secret:\n%s", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "tapi", "--rsh-operation", "signedReport", "--raw-header", "X-Partner-Key"}); err != nil {
		t.Fatalf("api auth inspect raw operation header: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "partner-secret" {
		t.Fatalf("raw header = %q, want partner-secret", got)
	}
}
