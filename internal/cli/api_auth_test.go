package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
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

func TestAPIAuthListJSONOutput(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {
              "auth": {"type": "api-key", "params": {"in": "header", "name": "X-Partner-Key", "value": "secret"}},
              "satisfies": ["items:read"]
            }
          }
        }
      }
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "list", "myapi", "-o", "json"}); err != nil {
		t.Fatalf("api auth list -o json: %v", err)
	}
	var got struct {
		API                        string `json:"api"`
		Profile                    string `json:"profile"`
		ProfileAuth                string `json:"profile_auth"`
		OperationMetadataAvailable bool   `json:"operation_metadata_available"`
		Credentials                []struct {
			ID        string   `json:"id"`
			Status    string   `json:"status"`
			Satisfies []string `json:"satisfies"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("parse JSON output: %v\n%s", err, out.String())
	}
	if got.API != "myapi" || got.Profile != "default" || got.ProfileAuth != "none" || got.OperationMetadataAvailable {
		t.Fatalf("auth list JSON = %#v", got)
	}
	if len(got.Credentials) != 1 || got.Credentials[0].ID != "PartnerKey" || got.Credentials[0].Status != "configured" {
		t.Fatalf("credentials JSON = %#v", got.Credentials)
	}
	if len(got.Credentials[0].Satisfies) != 1 || got.Credentials[0].Satisfies[0] != "items:read" {
		t.Fatalf("satisfies JSON = %#v", got.Credentials[0].Satisfies)
	}
}

func TestAPIAuthRemoveMissingCredentialFails(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {}
          }
        }
      }
    }
  }
}`)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	err := c.Run([]string{"restish", "api", "auth", "remove", "myapi", "Missing"})
	if err == nil {
		t.Fatal("expected missing credential removal to fail")
	}
	if !strings.Contains(err.Error(), `profile "default" of API "myapi" has no credential "Missing"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if written.APIs["myapi"].Profiles["default"].Credentials["PartnerKey"] == nil {
		t.Fatal("existing credential should remain")
	}
}

func TestAPIAuthConcurrentAddsPreserveCredentials(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "Existing": {}
          }
        }
      }
    }
  }
}`)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for _, credentialID := range []string{"RaceA", "RaceB"} {
		credentialID := credentialID
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, _, _ := newTestCLI(t)
			c.Hooks().ConfigPath = cfgFile
			errCh <- c.Run([]string{"restish", "api", "auth", "add", "myapi", credentialID})
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("api auth add: %v", err)
		}
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	creds := written.APIs["myapi"].Profiles["default"].Credentials
	for _, credentialID := range []string{"Existing", "RaceA", "RaceB"} {
		if creds[credentialID] == nil {
			t.Fatalf("credential %q missing after concurrent adds: %#v", credentialID, creds)
		}
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
	if !strings.Contains(got, "X-Partner-Key: secret") {
		t.Fatalf("expected API key inspection to show value, got %q", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--rsh-credential", "PartnerKey", "--redact"}); err != nil {
		t.Fatalf("api auth inspect --redact: %v", err)
	}
	got = out.String()
	if strings.Contains(got, "secret") || !strings.Contains(got, "X-Partner-Key: <redacted>") {
		t.Fatalf("expected redacted API key inspection with --redact, got %q", got)
	}
}

func TestAPIAuthInspectCredentialAPIKeyQuery(t *testing.T) {
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
                "params": {"in": "query", "name": "partner", "value": "secret"}
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
	if !strings.Contains(got, "Query: http://example.com?partner=secret") {
		t.Fatalf("expected API key query inspection to show value, got %q", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--rsh-credential", "PartnerKey", "--redact"}); err != nil {
		t.Fatalf("api auth inspect --redact: %v", err)
	}
	got = out.String()
	if strings.Contains(got, "secret") || !strings.Contains(got, "Query: http://example.com?partner=%3Credacted%3E") {
		t.Fatalf("expected redacted API key query inspection with --redact, got %q", got)
	}
}

func TestAPIAuthInspectURLSuggestsV2Form(t *testing.T) {
	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "api", "auth", "inspect", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected URL argument error")
	}
	if !strings.Contains(err.Error(), "v2 form: restish api auth header <api-name> Authorization") {
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
	if !strings.Contains(got, "X-Partner-Key: secret") {
		t.Fatalf("expected API key inspection to show value, got %q", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "header", "myapi", "X-Partner-Key"}); err != nil {
		t.Fatalf("api auth header: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "secret" {
		t.Fatalf("header = %q, want secret", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "header", "myapi", "X-Partner-Key", "PartnerKey"}); err != nil {
		t.Fatalf("api auth header with positional credential: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "secret" {
		t.Fatalf("header with positional credential = %q, want secret", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--raw-header", "X-Partner-Key"}); err != nil {
		t.Fatalf("api auth inspect raw header: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "secret" {
		t.Fatalf("raw header compatibility = %q, want secret", got)
	}
}

func TestAPIAuthInspectMultipleCredentialsShowsAll(t *testing.T) {
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

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi"}); err != nil {
		t.Fatalf("api auth inspect: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Credential: PartnerKey",
		"Auth type: api-key",
		"X-Partner-Key: secret",
		"Credential: UserBearer",
		"Auth type: bearer",
		"Authorization: Bearer user-token",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--redact"}); err != nil {
		t.Fatalf("api auth inspect --redact: %v", err)
	}
	got = out.String()
	for _, want := range []string{
		"Credential: PartnerKey",
		"X-Partner-Key: <redacted>",
		"Credential: UserBearer",
		"Authorization: <redacted>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("redacted inspect output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret") || strings.Contains(got, "user-token") {
		t.Fatalf("redacted inspect output leaked secret:\n%s", got)
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

func TestAPIAuthListReportsEnvReadinessAndProfileFallback(t *testing.T) {
	t.Setenv("READY_TOKEN", "ready")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "BearerAuth": {"type": "http", "scheme": "bearer"},
      "InferenceBearer": {"type": "http", "scheme": "bearer"},
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"}
    }
  },
  "paths": {
    "/me": {"get": {"operationId": "me", "security": [{"BearerAuth": []}], "responses": {"200": {"description": "OK"}}}},
    "/models": {"get": {"operationId": "models", "security": [{"InferenceBearer": []}], "responses": {"200": {"description": "OK"}}}},
    "/partner": {"get": {"operationId": "partner", "security": [{"PartnerKey": []}], "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	baseURL := readBaseURLFromConfig(t, env.cfgFile)
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {
			BaseURL: baseURL,
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					Auth: &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "env:READY_TOKEN"}},
					Credentials: map[string]*config.CredentialConfig{
						"BearerAuth": {
							Auth: &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "env:MISSING_TOKEN"}},
						},
					},
				},
			},
		},
	}})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "api", "auth", "list", "tapi"}); err != nil {
		t.Fatalf("api auth list: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Profile auth: configured",
		"Callable secured operations: 3/3",
		"BearerAuth: configured (env missing: MISSING_TOKEN)",
		"InferenceBearer: missing, satisfied by profile auth fallback",
		"PartnerKey: missing, satisfied by profile auth fallback (unchecked auth kind)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("list output missing %q:\n%s", want, got)
		}
	}
}

func TestAPIAuthInspectOperationLabelsProfileFallbackSource(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "InferenceBearer": {"type": "http", "scheme": "bearer"}
    }
  },
  "paths": {
    "/models": {"get": {"operationId": "listModels", "security": [{"InferenceBearer": []}], "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	baseURL := readBaseURLFromConfig(t, env.cfgFile)
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {
			BaseURL: baseURL,
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					Auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": "u", "password": "p"}},
				},
			},
		},
	}})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "tapi", "--rsh-operation", "list-models"}); err != nil {
		t.Fatalf("api auth inspect operation: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Operation: listModels",
		"Credentials: InferenceBearer",
		"Source: profile auth fallback",
		"Auth type: http-basic",
		"Authorization: Basic dTpw",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, got)
		}
	}
}

func TestAPIAuthListUsesImplicitDefaultProfile(t *testing.T) {
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
  "security": [{"PartnerKey": []}],
  "paths": {
    "/partner": {"get": {"operationId": "partnerReport", "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	baseURL := readBaseURLFromConfig(t, env.cfgFile)
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {BaseURL: baseURL},
	}})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "api", "auth", "list", "tapi"}); err != nil {
		t.Fatalf("api auth list: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Profile: default",
		"Profile auth: none",
		"Credentials: none",
		"PartnerKey: missing",
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

func TestAPIAuthInspectOperationFallsBackToRawSpecCache(t *testing.T) {
	dir := t.TempDir()
	specPath := dir + "/openapi.yaml"
	if err := os.WriteFile(specPath, []byte(`openapi: 3.0.3
info:
  title: Local Auth API
  version: "1.0"
servers:
  - url: http://127.0.0.1:8898
security:
  - bearer: []
components:
  securitySchemes:
    bearer:
      type: http
      scheme: bearer
paths:
  /private:
    get:
      operationId: privateEcho
      responses:
        "200": {description: OK}
  /public:
    get:
      operationId: publicEcho
      security: []
      responses:
        "200": {description: OK}
`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cfgFile := dir + "/restish.json"
	if err := os.WriteFile(cfgFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cacheDir := dir + "/spec-cache"

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "api", "connect", "controlauth", "http://127.0.0.1:8898", "--spec", specPath, "--replace", "prompt.credentials.bearer.token: local-secret"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "controlauth", "--rsh-operation", "privateEcho", "--raw-header", "Authorization"}); err != nil {
		t.Fatalf("api auth inspect private operation: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "Bearer local-secret" {
		t.Fatalf("Authorization = %q, want Bearer local-secret", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "controlauth", "--rsh-operation", "public-echo"}); err != nil {
		t.Fatalf("api auth inspect public operation: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Auth: none") {
		t.Fatalf("expected no-auth public operation, got:\n%s", got)
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
		"X-User-Key: user-secret",
		"X-Partner-Key: partner-secret",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "tapi", "--rsh-operation", "signedReport", "--redact"}); err != nil {
		t.Fatalf("api auth inspect redacted operation: %v", err)
	}
	got = out.String()
	for _, want := range []string{
		"X-User-Key: <redacted>",
		"X-Partner-Key: <redacted>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("redacted inspect output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "user-secret") || strings.Contains(got, "partner-secret") {
		t.Fatalf("redacted inspect output leaked secret:\n%s", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "header", "tapi", "X-Partner-Key", "--rsh-operation", "signedReport"}); err != nil {
		t.Fatalf("api auth operation header: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "partner-secret" {
		t.Fatalf("operation header = %q, want partner-secret", got)
	}
}
