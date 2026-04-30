package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/internal/cli"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

// specWithOperations returns an OpenAPI 3.1 spec JSON string.
func specWithOperations(baseURL string) string {
	return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0", "description": "# Test API\n\nGenerated **markdown** help."},
  "servers": [{"url": %q}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "summary": "List all items",
        "tags": ["items"],
        "parameters": [
          {
            "name": "limit",
            "in": "query",
            "required": false,
            "schema": {"type": "integer"},
            "description": "Max items to return"
          }
        ],
        "responses": {"200": {"description": "OK"}}
      },
      "post": {
        "operationId": "createItem",
        "summary": "Create an item",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {
            "type": "object",
            "properties": {
              "id": {"type": "string"},
              "amount": {"type": "string"},
              "name": {"type": "string"},
              "count": {"type": "integer"},
              "meta": {
                "type": "object",
                "properties": {"code": {"type": "string"}}
              }
            }
          }}}
        },
        "responses": {"201": {"description": "Created"}}
      }
    },
    "/items/{id}": {
      "get": {
        "operationId": "getItem",
        "summary": "Get item by ID",
        "tags": ["items"],
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {"type": "string"},
            "description": "The item ID"
          },
          {
            "name": "format",
            "in": "query",
            "required": false,
            "schema": {"type": "string"},
            "description": "Response format"
          }
        ],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/legacy": {
      "get": {
        "operationId": "getLegacy",
        "summary": "Deprecated endpoint",
        "deprecated": true,
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/public": {
      "get": {
        "operationId": "getPublic",
        "summary": "Public endpoint",
        "security": [],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/secret": {
      "get": {
        "operationId": "getSecret",
        "summary": "Hidden operation",
        "x-cli-hidden": true,
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
}

// generatedEnv holds shared state for generated-command tests.
type generatedEnv struct {
	cfgFile  string
	cacheDir string
}

// setupGeneratedEnv starts a test server using mux, registers /openapi.json on
// it, primes the spec cache, and returns an env that can be used to create
// multiple CLIs sharing the same config and cache.
func setupGeneratedEnv(t *testing.T, mux *http.ServeMux) *generatedEnv {
	t.Helper()

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	spec := specWithOperations(srv.URL)
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, spec)
	})

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o644)
	cacheDir := t.TempDir()

	env := &generatedEnv{cfgFile: cfgFile, cacheDir: cacheDir}

	// Prime the spec cache.
	c := env.newCLI()
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	if err := c.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}
	return env
}

// newCLI returns a fresh CLI sharing this env's config and cache.
// Stdout/Stderr default to io.Discard; callers may replace them.
func (e *generatedEnv) newCLI() *cli.CLI {
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = e.cfgFile
	c.Hooks().SpecCachePath = e.cacheDir
	c.Hooks().RetryBaseDelay = 0
	return c
}

// newCaptureCLI returns a CLI and a buffer capturing its combined output.
func (e *generatedEnv) newCaptureCLI() (*cli.CLI, *strings.Builder) {
	c := e.newCLI()
	var out strings.Builder
	c.Stdout = &out
	c.Stderr = &out
	return c, &out
}

type countingLoader struct {
	detects atomic.Int32
}

func (l *countingLoader) Detect(contentType string, body []byte) bool {
	l.detects.Add(1)
	return false
}

func (l *countingLoader) Load(body []byte) (*spec.APISpec, error) {
	return nil, nil
}

func (l *countingLoader) Priority() int {
	return 1000
}

func TestGeneratedAPIHelpUsesSpecDescriptionFromOperationCache(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c, out := env.newCaptureCLI()
	loader := &countingLoader{}
	c.AddLoader(loader)

	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("tapi --help: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Generated **markdown** help.") {
		t.Fatalf("expected API description in help, got:\n%s", got)
	}
	if strings.Contains(got, "Commands generated from the tapi API spec\n\nUsage:") {
		t.Fatalf("expected long help to replace generated placeholder, got:\n%s", got)
	}
	if got := loader.detects.Load(); got != 0 {
		t.Fatalf("loader Detect called %d times, want 0 when API help loads from cached operations", got)
	}
}

func TestGeneratedCommandUsesOperationCacheForExternalRefsOffline(t *testing.T) {
	var paramsAvailable atomic.Bool
	paramsAvailable.Store(true)
	var paramsHits atomic.Int32

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	root := fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "External Ref API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/items/{id}": {
      "get": {
        "operationId": "getItem",
        "parameters": [
          {"$ref": "./params.json#/components/parameters/ID"}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, srv.URL)
	params := `{
  "components": {
    "parameters": {
      "ID": {
        "name": "id",
        "in": "path",
        "required": true,
        "schema": {"type": "string"}
      }
    }
  }
}`

	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, root)
	})
	mux.HandleFunc("/params.json", func(w http.ResponseWriter, r *http.Request) {
		paramsHits.Add(1)
		if !paramsAvailable.Load() {
			http.Error(w, "params unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, params)
	})
	mux.HandleFunc("/items/abc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, cfgData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cacheDir := t.TempDir()
	env := &generatedEnv{cfgFile: cfgFile, cacheDir: cacheDir}

	syncCLI := env.newCLI()
	if err := syncCLI.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}
	if got := paramsHits.Load(); got == 0 {
		t.Fatal("expected api sync to fetch external params")
	}

	paramsAvailable.Store(false)
	hitsAfterSync := paramsHits.Load()
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-item", "abc"}); err != nil {
		t.Fatalf("generated command from operation cache: %v", err)
	}
	if got := paramsHits.Load(); got != hitsAfterSync {
		t.Fatalf("generated command refetched external params %d additional times", got-hitsAfterSync)
	}
}

// TestGeneratedCommandKebabCase verifies that operationId "listItems" becomes
// the command name "list-items".
func TestGeneratedCommandKebabCase(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items failed: %v", err)
	}
}

func TestGeneratedCommandSecurityEmptySuppressesAuth(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/public", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: strings.TrimSpace(readBaseURLFromConfig(t, env.cfgFile)),
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": "alice", "password": "secret"}},
					},
				},
			},
		},
	})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-public"}); err != nil {
		t.Fatalf("get-public failed: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty for security: [] operation", gotAuth)
	}
	err := c.Run([]string{"restish", "tapi", "get-public", "--rsh-auth", "PartnerKey"})
	if err == nil {
		t.Fatal("expected --rsh-auth to be rejected for security: [] operation")
	}
	if !strings.Contains(err.Error(), "security: []") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeneratedCommandDocumentSecurityLeavesProfileAuthBehavior(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/secure", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "BearerAuth": {"type": "http", "scheme": "bearer"}
    }
  },
  "security": [{"BearerAuth": []}],
  "paths": {
    "/secure": {
      "get": {
        "operationId": "getSecure",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: strings.TrimSpace(readBaseURLFromConfig(t, env.cfgFile)),
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": "alice", "password": "secret"}},
					},
				},
			},
		},
	})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-secure"}); err != nil {
		t.Fatalf("get-secure failed: %v", err)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("Authorization = %q, want Basic auth", gotAuth)
	}
}

func TestGeneratedCommandOAuthScopeAlternativesRequireCredentialBindings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/me/messages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Scope API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "GraphOAuth": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {
            "authorizationUrl": "https://login.example.com/authorize",
            "tokenUrl": "https://login.example.com/token",
            "scopes": {
              "Mail.Read": "Read mail",
              "User.Read": "Read users"
            }
          }
        }
      },
      "ApiKey": {"type": "apiKey", "in": "header", "name": "X-Api-Key"}
    }
  },
  "security": [{"GraphOAuth": ["User.Read"]}],
  "paths": {
    "/me/messages": {
      "get": {
        "operationId": "listMessages",
        "security": [
          {"GraphOAuth": ["Mail.Read"]},
          {"ApiKey": []}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: strings.TrimSpace(readBaseURLFromConfig(t, env.cfgFile)),
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Headers: []string{"Authorization: Bearer configured-token"},
					},
				},
			},
		},
	})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}

	c := env.newCLI()
	err := c.Run([]string{"restish", "tapi", "list-messages"})
	if err == nil {
		t.Fatal("expected missing credential binding error")
	}
	if !strings.Contains(err.Error(), "missing credential bindings") ||
		!strings.Contains(err.Error(), "GraphOAuth") ||
		!strings.Contains(err.Error(), "ApiKey") ||
		!strings.Contains(err.Error(), "restish api auth list tapi") ||
		!strings.Contains(err.Error(), "restish api auth add tapi <credential-id>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeneratedCommandUsesOperationCredentialBindings(t *testing.T) {
	got := map[string]http.Header{}
	mux := http.NewServeMux()
	for _, path := range []string{"/user", "/admin", "/partner", "/either", "/signed", "/public"} {
		path := path
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			got[path] = r.Header.Clone()
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{}`)
		})
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Security API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "UserOAuth": {"type": "oauth2", "flows": {"authorizationCode": {"authorizationUrl": "https://auth.example.com/authorize", "tokenUrl": "https://auth.example.com/token", "scopes": {"items:read": "Read items"}}}},
      "AdminOAuth": {"type": "oauth2", "flows": {"clientCredentials": {"tokenUrl": "https://auth.example.com/token", "scopes": {"admin:read": "Read admin"}}}},
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"}
    }
  },
  "security": [{"UserOAuth": ["items:read"]}],
  "paths": {
    "/user": {"get": {"operationId": "userItems", "responses": {"200": {"description": "OK"}}}},
    "/admin": {"get": {"operationId": "adminUsers", "security": [{"AdminOAuth": ["admin:read"]}], "responses": {"200": {"description": "OK"}}}},
    "/partner": {"get": {"operationId": "partnerReport", "security": [{"PartnerKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/either": {"get": {"operationId": "eitherReport", "security": [{"UserOAuth": ["items:read"]}, {"PartnerKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/signed": {"get": {"operationId": "signedReport", "security": [{"UserOAuth": ["items:read"], "PartnerKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/public": {"get": {"operationId": "status", "security": [], "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: strings.TrimSpace(readBaseURLFromConfig(t, env.cfgFile)),
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Profile-Key", "value": "profile"}},
						Credentials: map[string]*config.CredentialConfig{
							"UserOAuth": {
								Auth:      &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-User-Key", "value": "user"}},
								Satisfies: []string{"items:read"},
							},
							"AdminOAuth": {
								Auth:      &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Admin-Key", "value": "admin"}},
								Satisfies: []string{"admin:read"},
							},
							"PartnerKey": {
								Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Partner-Key", "value": "partner"}},
							},
						},
					},
				},
			},
		},
	})
	if err := os.WriteFile(env.cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}

	c := env.newCLI()
	for _, command := range []string{"user-items", "admin-users", "partner-report", "signed-report", "status"} {
		if err := c.Run([]string{"restish", "tapi", command}); err != nil {
			t.Fatalf("%s failed: %v", command, err)
		}
	}
	if err := c.Run([]string{"restish", "tapi", "either-report", "--rsh-auth", "PartnerKey"}); err != nil {
		t.Fatalf("either-report failed: %v", err)
	}

	if got["/user"].Get("X-User-Key") != "user" || got["/user"].Get("X-Profile-Key") != "" {
		t.Fatalf("/user headers = %#v", got["/user"])
	}
	if got["/admin"].Get("X-Admin-Key") != "admin" || got["/admin"].Get("X-User-Key") != "" {
		t.Fatalf("/admin headers = %#v", got["/admin"])
	}
	if got["/partner"].Get("X-Partner-Key") != "partner" || got["/partner"].Get("X-Profile-Key") != "" {
		t.Fatalf("/partner headers = %#v", got["/partner"])
	}
	if got["/either"].Get("X-Partner-Key") != "partner" || got["/either"].Get("X-User-Key") != "" {
		t.Fatalf("/either headers = %#v", got["/either"])
	}
	if got["/signed"].Get("X-User-Key") != "user" || got["/signed"].Get("X-Partner-Key") != "partner" {
		t.Fatalf("/signed headers = %#v", got["/signed"])
	}
	if got["/public"].Get("X-Profile-Key") != "" || got["/public"].Get("X-User-Key") != "" || got["/public"].Get("X-Partner-Key") != "" {
		t.Fatalf("/public headers = %#v", got["/public"])
	}
}

func readBaseURLFromConfig(t *testing.T, path string) string {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg.APIs["tapi"].BaseURL
}

func TestGeneratedCommandsLoadOnlyTargetAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	specDoc := specWithOperations(srv.URL)
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specDoc)
	})

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"alpha": {BaseURL: srv.URL},
			"beta":  {BaseURL: srv.URL},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, cfgData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cacheDir := t.TempDir()

	for _, name := range []string{"alpha", "beta"} {
		c := cli.New()
		c.Stdin = strings.NewReader("")
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		c.Hooks().ConfigPath = cfgFile
		c.Hooks().SpecCachePath = cacheDir
		c.Hooks().RetryBaseDelay = 0
		if err := c.Run([]string{"restish", "api", "sync", name}); err != nil {
			t.Fatalf("api sync %s: %v", name, err)
		}
	}

	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	c.Hooks().RetryBaseDelay = 0
	loader := &countingLoader{}
	c.AddLoader(loader)

	if err := c.Run([]string{"restish", "alpha", "list-items"}); err != nil {
		t.Fatalf("alpha list-items: %v", err)
	}
	if got := loader.detects.Load(); got != 0 {
		t.Fatalf("loader Detect called %d times, want 0 when generated commands load from cached operations", got)
	}
}

func TestBuiltinRequestDoesNotLoadGeneratedAPIs(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	if err := os.WriteFile(specPath, []byte(`openapi: "3.1.0"
info:
  title: Local
  version: "1.0.0"
paths: {}`), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"local": {
				BaseURL:   "https://api.example.com",
				SpecFiles: []string{specPath},
			},
		},
	})
	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(cfgFile, cfgData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	t.Cleanup(srv.Close)

	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	loader := &countingLoader{}
	c.AddLoader(loader)

	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("get URL: %v", err)
	}
	if got := loader.detects.Load(); got != 0 {
		t.Fatalf("loader Detect called %d times, want 0 for built-in request", got)
	}
}

// TestGeneratedCommandRequiredPathParam verifies that a required path param is
// a positional arg, is substituted in the URL, and that omitting it errors.
func TestGeneratedCommandRequiredPathParam(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/items/", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"abc"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)

	// With required arg: should succeed and hit /items/abc.
	c1 := env.newCLI()
	if err := c1.Run([]string{"restish", "tapi", "get-item", "abc"}); err != nil {
		t.Fatalf("get-item abc: %v", err)
	}
	if !strings.HasSuffix(lastPath, "/abc") {
		t.Errorf("expected path to end with /abc, got %q", lastPath)
	}

	// Without required arg: should fail (cobra MinimumNArgs check).
	c2 := env.newCLI()
	if err := c2.Run([]string{"restish", "tapi", "get-item"}); err == nil {
		t.Error("expected error when required arg is absent, got nil")
	} else if !strings.Contains(err.Error(), "missing required argument(s): id") ||
		!strings.Contains(err.Error(), "restish tapi get-item --help") {
		t.Fatalf("unexpected missing argument error: %v", err)
	}
}

// TestGeneratedCommandOptionalQueryFlag verifies that an optional query param
// becomes --flag and its value is sent as a URL query parameter.
func TestGeneratedCommandOptionalQueryFlag(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/items/", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-item", "x", "--format", "compact"}); err != nil {
		t.Fatalf("get-item x --format compact: %v", err)
	}
	if !strings.Contains(gotQuery, "format=compact") {
		t.Errorf("expected format=compact in query, got %q", gotQuery)
	}
}

// TestGeneratedCommandShorthandBody verifies that positional args after required
// params are parsed as shorthand and sent as the JSON request body.
func TestGeneratedCommandShorthandBody(t *testing.T) {
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotBody, _ = io.ReadAll(r.Body)
		}
		w.WriteHeader(201)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "create-item", "name:", "Widget"}); err != nil {
		t.Fatalf("create-item: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("body not valid JSON: %v — body: %s", err, gotBody)
	}
	if body["name"] != "Widget" {
		t.Errorf("name: got %v, want Widget", body["name"])
	}
}

func TestGeneratedCommandBodySchemaPreservesStringFields(t *testing.T) {
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotBody, _ = io.ReadAll(r.Body)
		}
		w.WriteHeader(201)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	err := c.Run([]string{
		"restish", "tapi", "create-item",
		"id:", "123,",
		"amount:", "9223372036854775807,",
		"count:", "7,",
		"meta.code:", "456,",
		"unknown:", "789",
	})
	if err != nil {
		t.Fatalf("create-item: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("body not valid JSON: %v — body: %s", err, gotBody)
	}
	for _, key := range []string{"id", "amount"} {
		if _, ok := body[key].(string); !ok {
			t.Fatalf("%s = %#v (%T), want string", key, body[key], body[key])
		}
	}
	if body["id"] != "123" {
		t.Fatalf("id = %#v, want string 123", body["id"])
	}
	if body["amount"] != "9223372036854775807" {
		t.Fatalf("amount = %#v, want string 9223372036854775807", body["amount"])
	}
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta = %T, want object", body["meta"])
	}
	if meta["code"] != "456" {
		t.Fatalf("meta.code = %#v, want string 456", meta["code"])
	}
	if _, ok := body["count"].(float64); !ok {
		t.Fatalf("count = %#v (%T), want JSON number", body["count"], body["count"])
	}
	if _, ok := body["unknown"].(float64); !ok {
		t.Fatalf("unknown = %#v (%T), want unknown fields left as parsed numbers", body["unknown"], body["unknown"])
	}
}

func TestGeneratedCommandGenerateBodyPrintsExampleWithoutRequest(t *testing.T) {
	var hit bool
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(500)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "create-item", "--rsh-generate-body"}); err != nil {
		t.Fatalf("generate body: %v", err)
	}
	if hit {
		t.Fatal("generate body should not send a request")
	}
	got := out.String()
	for _, want := range []string{`"id": "string"`, `"amount": "string"`, `"count": 1`} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated body missing %q:\n%s", want, got)
		}
	}
}

// TestGeneratedCommandHelp verifies that --help shows parameter descriptions
// from the OpenAPI spec.
func TestGeneratedCommandHelp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c, out := env.newCaptureCLI()
	_ = c.Run([]string{"restish", "tapi", "get-item", "--help"})
	got := out.String()
	if !strings.Contains(got, "The item ID") {
		t.Errorf("expected parameter description in help output, got:\n%s", got)
	}
	if !strings.Contains(got, "format") {
		t.Errorf("expected --format flag in help output, got:\n%s", got)
	}
}

func TestGeneratedCommandHelpFocusesOperationAndHelpAllShowsGlobals(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-item", "--help"}); err != nil {
		t.Fatalf("focused help: %v", err)
	}
	focused := out.String()
	if strings.Contains(focused, "Global Flags:") || strings.Contains(focused, "--rsh-header") {
		t.Fatalf("focused help should hide global flags, got:\n%s", focused)
	}
	if !strings.Contains(focused, "--help-all") {
		t.Fatalf("focused help should point to --help-all, got:\n%s", focused)
	}
	if got := strings.Count(focused, "--help-all"); got != 1 {
		t.Fatalf("focused help should show --help-all once, got %d occurrences:\n%s", got, focused)
	}
	generalIdx := strings.Index(focused, "General Options")
	helpAllIdx := strings.Index(focused, "--help-all")
	optionsIdx := strings.Index(focused, "\nOptions\n")
	if generalIdx < 0 || optionsIdx < 0 || helpAllIdx < generalIdx || helpAllIdx > optionsIdx {
		t.Fatalf("focused help should group --help-all under General Options before operation options:\n%s", focused)
	}

	c, out = env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-item", "--help-all"}); err != nil {
		t.Fatalf("help-all: %v", err)
	}
	full := out.String()
	if !strings.Contains(full, "Global Flags:") || !strings.Contains(full, "--rsh-header") {
		t.Fatalf("help-all should include inherited global flags, got:\n%s", full)
	}
}

func TestGeneratedCommandLayoutTagsCreatesTagSubcommands(t *testing.T) {
	var hit bool
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	cfg, err := config.Load(env.cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.APIs["tapi"].CommandLayout = "tags"
	if err := config.Save(env.cfgFile, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("tapi --help: %v", err)
	}
	if !strings.Contains(out.String(), "items") {
		t.Fatalf("expected tag subcommand in API help, got:\n%s", out.String())
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "items", "list-items"}); err != nil {
		t.Fatalf("tagged list-items failed: %v", err)
	}
	if !hit {
		t.Fatal("tagged operation did not send request")
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err == nil {
		t.Fatal("flat command should not be registered when command_layout is tags")
	}
}

func TestGeneratedCommandHelpShowsSchemasExamplesAndGroupedErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Pets", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "schemas": {
      "Pet": {
        "type": "object",
        "required": ["id", "name"],
        "properties": {
          "id": {"type": "string", "readOnly": true},
          "name": {"type": "string"},
          "secret_token": {"type": "string", "writeOnly": true}
        }
      },
      "ErrorModel": {
        "type": "object",
        "required": ["message"],
        "properties": {
          "message": {"type": "string"}
        }
      }
    }
  },
  "paths": {
    "/pets": {
      "post": {
        "operationId": "createPet",
        "summary": "Create a pet",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "example": {"name": "Fluffy", "secret_token": "abc123"},
              "schema": {"$ref": "#/components/schemas/Pet"}
            }
          }
        },
        "responses": {
          "201": {
            "description": "Created",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Pet"}}}
          },
          "400": {
            "description": "Bad request",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ErrorModel"}}}
          },
          "404": {
            "description": "Not found",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ErrorModel"}}}
          },
          "500": {
            "description": "Inline equivalent error",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "required": ["message"],
                  "properties": {"message": {"type": "string"}}
                }
              }
            }
          },
          "default": {
            "description": "Default error",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/ErrorModel"}}}
          }
        }
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "create-pet", "--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Input Example:",
		`"name": "Fluffy"`,
		`"secret_token": "abc123"`,
		"Request Schema (application/json):",
		"name*: (string)",
		"Response 201 (application/json):",
		"id*: (string)",
		"Responses 400/404/500/default (application/json):",
		"message*: (string)",
		"restish tapi create-pet name: Fluffy, secret_token: abc123",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected help to contain %q, got:\n%s", want, got)
		}
	}
}

func TestGeneratedCommandHelpShowsResponseHeaders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Headers", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "responses": {
          "200": {
            "description": "A paged array of items",
            "headers": {
              "Next": {
                "description": "A link to the next page of responses",
                "schema": {"type": "string"}
              }
            },
            "content": {
              "application/json": {
                "schema": {"type": "array", "items": {"type": "string"}}
              }
            }
          }
        }
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "list-items", "--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Headers: Next") {
		t.Fatalf("expected response header names in help, got:\n%s", got)
	}
}

func TestGeneratedCommandHelpBoundsRecursiveSchemas(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Tree", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "schemas": {
      "Node": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "child": {"$ref": "#/components/schemas/Node"}
        }
      }
    }
  },
  "paths": {
    "/tree": {
      "get": {
        "operationId": "getTree",
        "responses": {
          "200": {
            "description": "OK",
            "content": {"application/json": {"schema": {"$ref": "#/components/schemas/Node"}}}
          }
        }
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-tree", "--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "<recursive ref>") {
		t.Fatalf("expected recursive schema marker, got:\n%s", got)
	}
}

func TestGeneratedCommandHelpShowsCompositeSchemaMetadata(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Composite", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "schemas": {
      "Base": {
        "type": "object",
        "properties": {
          "id": {"type": "string", "format": "uuid", "default": "00000000-0000-0000-0000-000000000000"},
          "state": {"type": "string", "enum": ["new", "done"]},
          "kind": {"type": "string", "const": "item"},
          "score": {"type": "number", "minimum": 5, "maximum": 10, "multipleOf": 0.5},
          "age": {"type": "integer", "exclusiveMinimum": 0, "exclusiveMaximum": 120}
        }
      },
      "Extra": {
        "type": "object",
        "additionalProperties": {"type": "integer"}
      }
    }
  },
  "paths": {
    "/items": {
      "post": {
        "operationId": "createComposite",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "allOf": [
                  {"$ref": "#/components/schemas/Base"},
                  {"type": "object", "properties": {"name": {"type": "string", "examples": ["Alpha"]}}}
                ]
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/json": {
                "schema": {
                  "oneOf": [
                    {"$ref": "#/components/schemas/Base"},
                    {"$ref": "#/components/schemas/Extra"}
                  ],
                  "discriminator": {"propertyName": "kind"}
                }
              }
            }
          },
          "400": {
            "description": "Queued",
            "content": {
              "application/json": {
                "schema": {
                  "anyOf": [
                    {"type": "object", "properties": {"queued": {"type": "boolean"}}},
                    {"$ref": "#/components/schemas/Extra"}
                  ]
                }
              }
            }
          }
        }
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "create-composite", "--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"allOf{",
		"oneOf{",
		"anyOf{",
		"format:uuid",
		"default:00000000-0000-0000-0000-000000000000",
		"enum:new,done",
		"const:item",
		"min:5",
		"max:10",
		"multiple:0.5",
		"exclusiveMin:0",
		"exclusiveMax:120",
		`"name": "Alpha"`,
		"<any>: (integer)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected help to contain %q, got:\n%s", want, got)
		}
	}
}

func TestGeneratedCommandMissingOperationIDFallsBackToMethodAndPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/widgets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/widgets": {
      "get": {
        "summary": "List widgets",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-widgets"}); err != nil {
		t.Fatalf("get-widgets failed: %v", err)
	}
}

func TestGeneratedCommandMissingOperationIDFallbackSlugIsSafe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users/123/repos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/users/{user_id}/repos": {
      "get": {
        "parameters": [
          {"name": "user_id", "in": "path", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-users-user-id-repos", "123"}); err != nil {
		t.Fatalf("safe fallback command failed: %v", err)
	}
}

func TestGeneratedCommandTypedQueryFlagsAndStyles(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/search": {
      "get": {
        "operationId": "search",
        "parameters": [
          {"name": "includeArchived", "in": "query", "schema": {"type": "boolean"}},
          {"name": "limit", "in": "query", "schema": {"type": "integer", "default": 25}},
          {"name": "score", "in": "query", "schema": {"type": "number"}},
          {"name": "tag", "in": "query", "style": "form", "explode": true, "schema": {"type": "array", "items": {"type": "string"}}},
          {"name": "ids", "in": "query", "style": "form", "explode": false, "schema": {"type": "array", "items": {"type": "string"}}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "search", "--include-archived", "--score", "1.5", "--tag", "red", "--tag", "blue", "--ids", "1", "--ids", "2"}); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("includeArchived"); got != "true" {
		t.Fatalf("includeArchived = %q, want true", got)
	}
	if got := values.Get("limit"); got != "25" {
		t.Fatalf("limit default = %q, want 25", got)
	}
	if got := values.Get("score"); got != "1.5" {
		t.Fatalf("score = %q, want 1.5", got)
	}
	if got := values["tag"]; strings.Join(got, ",") != "red,blue" {
		t.Fatalf("tag values = %v, want red and blue", got)
	}
	if got := values.Get("ids"); got != "1,2" {
		t.Fatalf("ids = %q, want 1,2", got)
	}
}

func TestGeneratedCommandAnyOfQueryParamUsesStringFlag(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(200)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/search": {
      "get": {
        "operationId": "search",
        "parameters": [
          {"name": "value", "in": "query", "schema": {"anyOf": [{"type": "string"}, {"type": "integer"}]}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "search", "--value", "abc123"}); err != nil {
		t.Fatalf("search failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("value"); got != "abc123" {
		t.Fatalf("value = %q, want abc123", got)
	}
}

func TestGeneratedCommandODataQueryParameterNames(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "OData API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/users": {
      "get": {
        "operationId": "listUsers",
        "parameters": [
          {"name": "$select", "in": "query", "schema": {"type": "string"}},
          {"name": "$filter", "in": "query", "schema": {"type": "string"}},
          {"name": "$top", "in": "query", "schema": {"type": "integer"}},
          {"name": "$orderby", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{
		"restish", "tapi", "list-users",
		"--select", "id,displayName",
		"--filter", "startswith(displayName,'A')",
		"--top", "25",
		"--orderby", "displayName desc",
	}); err != nil {
		t.Fatalf("list-users failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	for key, want := range map[string]string{
		"$select":  "id,displayName",
		"$filter":  "startswith(displayName,'A')",
		"$top":     "25",
		"$orderby": "displayName desc",
	} {
		if got := values.Get(key); got != want {
			t.Fatalf("%s = %q, want %q; raw=%q", key, got, want, gotQuery)
		}
	}
}

func TestGeneratedCommandUsesRequestBodyMediaType(t *testing.T) {
	var gotContentType, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/submit": {
      "post": {
        "operationId": "submit",
        "requestBody": {
          "content": {
            "application/x-www-form-urlencoded": {"schema": {"type": "object"}},
            "text/plain": {"schema": {"type": "string"}}
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "submit", "name:", "Widget"}); err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	if !strings.HasPrefix(gotContentType, "application/x-www-form-urlencoded") {
		t.Fatalf("Content-Type = %q, want form", gotContentType)
	}
	if gotBody != "name=Widget" {
		t.Fatalf("body = %q, want form body", gotBody)
	}
}

func TestGeneratedCommandFormRequestBodyNestedArrays(t *testing.T) {
	var gotContentType, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/charges", func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Stripe-ish API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/charges": {
      "post": {
        "operationId": "createCharge",
        "requestBody": {
          "content": {
            "application/x-www-form-urlencoded": {
              "schema": {
                "type": "object",
                "properties": {
                  "amount": {"type": "integer"},
                  "capture": {"type": "boolean"},
                  "expand": {"type": "array", "items": {"type": "string"}},
                  "metadata": {
                    "type": "object",
                    "additionalProperties": {"type": "string"}
                  }
                }
              }
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{
		"restish", "tapi", "create-charge",
		"amount:", "2000,",
		"capture:", "false,",
		"expand:", "[customer,invoice],",
		"metadata.order_id:", "ord_123",
	}); err != nil {
		t.Fatalf("create-charge failed: %v", err)
	}
	if !strings.HasPrefix(gotContentType, "application/x-www-form-urlencoded") {
		t.Fatalf("Content-Type = %q, want form", gotContentType)
	}
	values, err := url.ParseQuery(gotBody)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotBody, err)
	}
	if got := values.Get("amount"); got != "2000" {
		t.Fatalf("amount = %q, want 2000", got)
	}
	if got := values.Get("capture"); got != "false" {
		t.Fatalf("capture = %q, want false", got)
	}
	if got := values["expand[]"]; strings.Join(got, ",") != "customer,invoice" {
		t.Fatalf("expand[] = %#v, want customer and invoice", got)
	}
	if got := values.Get("metadata[order_id]"); got != "ord_123" {
		t.Fatalf("metadata[order_id] = %q, want ord_123; body=%q", got, gotBody)
	}
}

func TestGeneratedCommandMultipartRequestBody(t *testing.T) {
	var gotContentType string
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/upload": {
      "post": {
        "operationId": "uploadItem",
        "requestBody": {
          "content": {
            "multipart/form-data": {
              "schema": {
                "type": "object",
                "properties": {
                  "name": {"type": "string"},
                  "meta": {"type": "object", "properties": {"role": {"type": "string"}}},
                  "file": {"type": "string", "format": "binary"},
                  "files": {
                    "type": "array",
                    "items": {"type": "string", "format": "binary"}
                  }
                }
              },
              "encoding": {
                "meta": {"contentType": "application/json"},
                "file": {"contentType": "text/plain"},
                "files": {"contentType": "text/plain"}
              }
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	uploadPath := filepath.Join("testdata", "upload.txt")
	c := env.newCLI()
	if err := c.Run([]string{
		"restish", "tapi", "upload-item",
		"name:", "alice,",
		"meta.role:", "avatar,",
		"file:", "@" + uploadPath + ",",
		"files:", "[@" + uploadPath + ",@" + uploadPath + "]",
	}); err != nil {
		t.Fatalf("upload-item failed: %v", err)
	}

	mediaType, params, err := mime.ParseMediaType(gotContentType)
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}
	if mediaType != "multipart/form-data" {
		t.Fatalf("Content-Type = %q, want multipart/form-data", gotContentType)
	}
	reader := multipart.NewReader(bytes.NewReader(gotBody), params["boundary"])
	parts := map[string][]string{}
	filenames := map[string][]string{}
	partContentTypes := map[string][]string{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		name := part.FormName()
		parts[name] = append(parts[name], string(content))
		filenames[name] = append(filenames[name], part.FileName())
		partContentTypes[name] = append(partContentTypes[name], part.Header.Get("Content-Type"))
	}
	if got := parts["name"]; len(got) != 1 || got[0] != "alice" {
		t.Fatalf("name part = %#v, want alice", got)
	}
	if got := parts["meta"]; len(got) != 1 || got[0] != `{"role":"avatar"}` {
		t.Fatalf("meta part = %#v", got)
	}
	if got := partContentTypes["meta"]; len(got) != 1 || got[0] != "application/json" {
		t.Fatalf("meta content type = %#v, want application/json", got)
	}
	if got := parts["file"]; len(got) != 1 || got[0] != "hello from upload\n" {
		t.Fatalf("file part = %#v", got)
	}
	if got := filenames["file"]; len(got) != 1 || got[0] != "upload.txt" {
		t.Fatalf("file name = %#v, want upload.txt", got)
	}
	if got := partContentTypes["file"]; len(got) != 1 || got[0] != "text/plain" {
		t.Fatalf("file content type = %#v, want text/plain", got)
	}
	if got := parts["files"]; len(got) != 2 || got[0] != "hello from upload\n" || got[1] != "hello from upload\n" {
		t.Fatalf("files parts = %#v", got)
	}
	if got := filenames["files"]; len(got) != 2 || got[0] != "upload.txt" || got[1] != "upload.txt" {
		t.Fatalf("files names = %#v, want repeated upload.txt", got)
	}
}

func TestGeneratedCommandOctetStreamRequestBody(t *testing.T) {
	var gotContentType, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/blob", func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/blob": {
      "post": {
        "operationId": "putBlob",
        "requestBody": {
          "content": {
            "application/octet-stream": {
              "schema": {"type": "string", "format": "binary"}
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "put-blob", "raw-bytes"}); err != nil {
		t.Fatalf("put-blob failed: %v", err)
	}
	if !strings.HasPrefix(gotContentType, "application/octet-stream") {
		t.Fatalf("Content-Type = %q, want application/octet-stream", gotContentType)
	}
	if gotBody != "raw-bytes" {
		t.Fatalf("body = %q, want raw-bytes", gotBody)
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "put-blob", "@" + filepath.Join("testdata", "upload.txt")}); err != nil {
		t.Fatalf("put-blob file failed: %v", err)
	}
	if gotBody != "hello from upload\n" {
		t.Fatalf("file body = %q", gotBody)
	}
}

func TestGeneratedCommandGETRequestBody(t *testing.T) {
	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/search": {
      "get": {
        "operationId": "searchWithBody",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"type": "object", "properties": {"q": {"type": "string"}}}
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "search-with-body", "q:", "widgets"}); err != nil {
		t.Fatalf("search-with-body failed: %v", err)
	}
	if gotBody != `{"q":"widgets"}` {
		t.Fatalf("body = %q, want JSON body", gotBody)
	}
}

func TestGeneratedCommandVendorJSONRequestBody(t *testing.T) {
	var gotContentType, gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos", func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.Header().Set("Content-Type", "application/vnd.github+json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Vendor JSON API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/repos": {
      "post": {
        "operationId": "createRepo",
        "requestBody": {
          "content": {
            "application/vnd.github+json": {
              "schema": {
                "type": "object",
                "properties": {"name": {"type": "string"}}
              }
            }
          }
        },
        "responses": {"201": {"description": "Created"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "create-repo", "name:", "restish"}); err != nil {
		t.Fatalf("create-repo failed: %v", err)
	}
	if !strings.HasPrefix(gotContentType, "application/vnd.github+json") {
		t.Fatalf("Content-Type = %q, want vendor +json", gotContentType)
	}
	if gotBody != `{"name":"restish"}` {
		t.Fatalf("body = %q, want JSON body", gotBody)
	}
}

func TestGeneratedCommandHelpShowsNonJSONResponseMedia(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Diff API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/repos/{owner}/{repo}/compare/{basehead}": {
      "get": {
        "operationId": "compareCommits",
        "parameters": [
          {"name": "owner", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "repo", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "basehead", "in": "path", "required": true, "x-multi-segment": true, "schema": {"type": "string"}}
        ],
        "responses": {
          "200": {
            "description": "Diff",
            "content": {
              "application/vnd.github.diff": {},
              "application/vnd.github.patch": {},
              "text/plain": {}
            }
          }
        }
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "compare-commits", "--help"}); err != nil {
		t.Fatalf("compare-commits --help failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Response 200 (application/vnd.github.diff):",
		"Response has no body",
		"compare-commits <owner> <repo> <basehead>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q:\n%s", want, got)
		}
	}
}

func TestGeneratedCommandTraceOperation(t *testing.T) {
	var gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Trace API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/diagnostics": {
      "trace": {
        "operationId": "traceDiagnostics",
        "responses": {"204": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "trace-diagnostics"}); err != nil {
		t.Fatalf("trace-diagnostics failed: %v", err)
	}
	if gotMethod != http.MethodTrace {
		t.Fatalf("method = %q, want TRACE", gotMethod)
	}
}

func TestGeneratedCommandIgnoresReservedHeaderParameters(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Reserved Header API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "parameters": [
          {"name": "Authorization", "in": "header", "required": true, "schema": {"type": "string"}},
          {"name": "Accept", "in": "header", "schema": {"type": "string"}},
          {"name": "Content-Type", "in": "header", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})
	cfg, err := config.Load(env.cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.APIs["tapi"].Profiles = map[string]*config.ProfileConfig{
		"default": {Headers: []string{"Authorization: Bearer profile-token"}},
	}
	if err := config.Save(env.cfgFile, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items should not require reserved header args: %v", err)
	}
	if gotAuth != "Bearer profile-token" {
		t.Fatalf("Authorization = %q, want profile auth header", gotAuth)
	}
}

func TestGeneratedCommandUsesPathItemParameters(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/tenants/", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/tenants/{tenant}/items": {
      "parameters": [
        {
          "name": "tenant",
          "in": "path",
          "required": true,
          "schema": {"type": "string"},
          "description": "Tenant name"
        }
      ],
      "get": {
        "operationId": "listTenantItems",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-tenant-items", "acme"}); err != nil {
		t.Fatalf("list-tenant-items acme failed: %v", err)
	}
	if lastPath != "/tenants/acme/items" {
		t.Fatalf("expected substituted path, got %q", lastPath)
	}
}

func TestGeneratedCommandRequiredHeaderIsRequiredArgument(t *testing.T) {
	var authHeader string
	mux := http.NewServeMux()
	mux.HandleFunc("/secure", func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("X-Auth")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/secure": {
      "get": {
        "operationId": "getSecure",
        "parameters": [
          {
            "name": "X-Auth",
            "in": "header",
            "required": true,
            "schema": {"type": "string"},
            "description": "Auth token"
          }
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c1 := env.newCLI()
	if err := c1.Run([]string{"restish", "tapi", "get-secure"}); err == nil {
		t.Fatal("expected missing required header argument to error")
	}

	c2 := env.newCLI()
	if err := c2.Run([]string{"restish", "tapi", "get-secure", "secret"}); err != nil {
		t.Fatalf("get-secure with required header argument failed: %v", err)
	}
	if authHeader != "secret" {
		t.Fatalf("expected X-Auth header to be sent, got %q", authHeader)
	}
}

func TestGeneratedCommandRequiredNonPathParamsAppearInHelp(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/tenants/{tenant}/reports": {
      "get": {
        "operationId": "getReport",
        "parameters": [
          {"name": "tenant", "in": "path", "required": true, "description": "Tenant ID", "schema": {"type": "string"}},
          {"name": "account", "in": "query", "required": true, "description": "Account ID", "schema": {"type": "string"}},
          {"name": "X-Auth", "in": "header", "required": true, "description": "Auth token", "schema": {"type": "string"}},
          {"name": "session", "in": "cookie", "required": true, "description": "Session token", "schema": {"type": "string"}},
          {"name": "page", "in": "query", "description": "Page number", "schema": {"type": "integer"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-report", "--help"}); err != nil {
		t.Fatalf("get-report --help: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Usage:",
		"get-report <tenant> <account> <x-auth> <session>",
		"Arguments:",
		"tenant",
		"Tenant ID",
		"account",
		"Account ID",
		"x-auth",
		"Auth token",
		"session",
		"Session token",
		"--page",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("help missing %q:\n%s", want, got)
		}
	}
}

func TestGeneratedCommandRequiredQueryIsRequiredArgument(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/reports", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/reports": {
      "get": {
        "operationId": "listReports",
        "parameters": [
          {"name": "account", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c1 := env.newCLI()
	if err := c1.Run([]string{"restish", "tapi", "list-reports"}); err == nil {
		t.Fatal("expected missing required query argument to error")
	}

	c2 := env.newCLI()
	if err := c2.Run([]string{"restish", "tapi", "list-reports", "acme"}); err != nil {
		t.Fatalf("list-reports with required query argument failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("account"); got != "acme" {
		t.Fatalf("account = %q, want acme", got)
	}
}

func TestGeneratedCommandRequiredCookieIsRequiredArgument(t *testing.T) {
	var sessionValue string
	mux := http.NewServeMux()
	mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("session"); err == nil {
			sessionValue = cookie.Value
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/session": {
      "get": {
        "operationId": "getSession",
        "parameters": [
          {"name": "session", "in": "cookie", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c1 := env.newCLI()
	if err := c1.Run([]string{"restish", "tapi", "get-session"}); err == nil {
		t.Fatal("expected missing required cookie argument to error")
	}

	c2 := env.newCLI()
	if err := c2.Run([]string{"restish", "tapi", "get-session", "abc123"}); err != nil {
		t.Fatalf("get-session with required cookie argument failed: %v", err)
	}
	if sessionValue != "abc123" {
		t.Fatalf("session cookie = %q, want abc123", sessionValue)
	}
}

func TestGeneratedCommandSameNamePathAndQueryParams(t *testing.T) {
	var gotPath, gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/items/path-id", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/items/{id}": {
      "get": {
        "operationId": "getItem",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "id", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-item", "path-id", "--id", "query-id"}); err != nil {
		t.Fatalf("get-item failed: %v", err)
	}
	if gotPath != "/items/path-id" {
		t.Fatalf("path = %q, want /items/path-id", gotPath)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("id"); got != "query-id" {
		t.Fatalf("query id = %q, want query-id", got)
	}
}

func TestGeneratedCommandMultiSegmentPathParamIsEscaped(t *testing.T) {
	var gotRequestURI string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/octocat/hello-world/compare/", func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "GitHub-ish API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/repos/{owner}/{repo}/compare/{basehead}": {
      "get": {
        "operationId": "compareCommits",
        "parameters": [
          {"name": "owner", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "repo", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "basehead", "in": "path", "required": true, "x-multi-segment": true, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "compare-commits", "octocat", "hello-world", "main...feature/slashy"}); err != nil {
		t.Fatalf("compare-commits failed: %v", err)
	}
	if !strings.Contains(gotRequestURI, "/repos/octocat/hello-world/compare/main...feature%2Fslashy") {
		t.Fatalf("RequestURI = %q, want slash escaped in x-multi-segment path param", gotRequestURI)
	}
}

func TestGeneratedCommandNameCollisionsAreDisambiguated(t *testing.T) {
	var getHits, postHits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getHits.Add(1)
		case http.MethodPost:
			postHits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "responses": {"200": {"description": "OK"}}
      },
      "post": {
        "operationId": "list-items",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	if !strings.Contains(out.String(), "list-items-post") {
		t.Fatalf("expected disambiguated command name in help, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "warning: command name collision") {
		t.Fatalf("expected collision warning in output, got:\n%s", out.String())
	}

	c1 := env.newCLI()
	if err := c1.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items failed: %v", err)
	}
	c2 := env.newCLI()
	if err := c2.Run([]string{"restish", "tapi", "list-items-post"}); err != nil {
		t.Fatalf("list-items-post failed: %v", err)
	}
	if getHits.Load() != 1 || postHits.Load() != 1 {
		t.Fatalf("expected both commands to remain callable, got GET=%d POST=%d", getHits.Load(), postHits.Load())
	}
}

func TestGeneratedCommandLargePublicSpecShapeSmoke(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		var paths strings.Builder
		for i := 0; i < 150; i++ {
			if i > 0 {
				paths.WriteString(",")
			}
			fmt.Fprintf(&paths, `"/resources/%03d": {
      "get": {
        "operationId": "getResource%03d",
        "tags": ["resources"],
        "parameters": [],
        "responses": {"200": {"description": "OK"}}
      }
    }`, i, i)
		}
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Large API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {%s}
}`, baseURL, paths.String())
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("large spec help failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"get-resource000", "get-resource149", "resources"} {
		if !strings.Contains(got, want) {
			t.Fatalf("large spec help missing %q:\n%s", want, got)
		}
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-resource149"}); err != nil {
		t.Fatalf("large spec generated command failed: %v", err)
	}
}

func TestGeneratedCommandsRespectServersBasePath(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/items", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": "/v1"}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items failed: %v", err)
	}
	if lastPath != "/v1/items" {
		t.Fatalf("expected servers base path to be applied, got %q", lastPath)
	}
}

func TestGeneratedCommandsRespectPathAndOperationServers(t *testing.T) {
	var pathHit, operationHit bool
	mux := http.NewServeMux()
	mux.HandleFunc("/path-base/items", func(w http.ResponseWriter, r *http.Request) {
		pathHit = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/op-base/widgets", func(w http.ResponseWriter, r *http.Request) {
		operationHit = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return `{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": "/doc-base"}],
  "paths": {
    "/items": {
      "servers": [{"url": "/path-base"}],
      "get": {
        "operationId": "listItems",
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/widgets": {
      "servers": [{"url": "/path-base"}],
      "get": {
        "operationId": "listWidgets",
        "servers": [{"url": "/op-base"}],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items failed: %v", err)
	}
	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-widgets"}); err != nil {
		t.Fatalf("list-widgets failed: %v", err)
	}
	if !pathHit {
		t.Fatal("path-level server base was not used")
	}
	if !operationHit {
		t.Fatal("operation-level server base was not used")
	}
}

func TestGeneratedCommandsResolveRelativeServerURLAgainstAPIBase(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": "{version}", "variables": {"version": {"default": "v2"}}}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specBody)
	})
	mux.HandleFunc("/root/v2/items", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL + "/root", SpecURL: srv.URL + "/openapi.json"},
		},
	})
	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cacheDir := t.TempDir()

	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}

	c = cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items failed: %v", err)
	}
	if lastPath != "/root/v2/items" {
		t.Fatalf("expected relative server URL to resolve against API base path, got %q", lastPath)
	}
}

func TestGeneratedCommandsResolveRootRelativeServerURLEscapingAPIBase(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": "/v1"}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specBody)
	})
	mux.HandleFunc("/v1/items", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/root/v1/items", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server URL should escape API base path, got %s", r.URL.Path)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL + "/root", SpecURL: srv.URL + "/openapi.json"},
		},
	})
	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cacheDir := t.TempDir()

	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}

	c = cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items failed: %v", err)
	}
	if lastPath != "/v1/items" {
		t.Fatalf("expected root-relative server URL to escape API base path, got %q", lastPath)
	}
}

func TestGeneratedCommandsResolveOperationBasePathAgainstBaseURL(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/my-op", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/api/v2-beta1/my-op", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("operation_base should escape the API base path, got %s", r.URL.Path)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/my-op": {
      "get": {
        "operationId": "myOp",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL+"/api/v2-beta1")
	})
	cfg, err := config.Load(env.cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	baseURL := cfg.APIs["tapi"].BaseURL
	cfg.APIs["tapi"].BaseURL = baseURL + "/api/v2-beta1"
	cfg.APIs["tapi"].OperationBase = "/"
	if err := config.Save(env.cfgFile, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "my-op"}); err != nil {
		t.Fatalf("my-op failed: %v", err)
	}
	if lastPath != "/my-op" {
		t.Fatalf("expected operation_base path to resolve against base_url root, got %q", lastPath)
	}
}

func TestGeneratedCommandsReloadLocalSpecFilesWhenChanged(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/widgets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specMtime := time.Unix(1700000000, 0)
	writeSpec := func(body string) {
		t.Helper()
		if err := os.WriteFile(specPath, []byte(body), 0o644); err != nil {
			t.Fatalf("write spec: %v", err)
		}
		specMtime = specMtime.Add(time.Second)
		if err := os.Chtimes(specPath, specMtime, specMtime); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	writeSpec(fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/items": {
      "get": {
        "operationId": "listItems",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, srv.URL))

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL, SpecFiles: []string{specPath}},
		},
	})
	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(cfgFile, cfgData, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cacheDir := t.TempDir()

	c1 := cli.New()
	c1.Stdin = strings.NewReader("")
	c1.Stdout = io.Discard
	c1.Stderr = io.Discard
	c1.Hooks().ConfigPath = cfgFile
	c1.Hooks().SpecCachePath = cacheDir
	if err := c1.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}

	specMtime = time.Now().Add(time.Hour)
	writeSpec(fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/widgets": {
      "get": {
        "operationId": "getWidgets",
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, srv.URL))

	c2 := cli.New()
	c2.Stdin = strings.NewReader("")
	c2.Stdout = io.Discard
	c2.Stderr = io.Discard
	c2.Hooks().ConfigPath = cfgFile
	c2.Hooks().SpecCachePath = cacheDir
	if err := c2.Run([]string{"restish", "tapi", "get-widgets"}); err != nil {
		t.Fatalf("expected updated local spec to be reflected without sync: %v", err)
	}
}

func TestGeneratedCommandWithoutBodyRejectsExtraArgs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"abc"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	err := c.Run([]string{"restish", "tapi", "get-item", "abc", "unexpected"})
	if err == nil {
		t.Fatal("expected extra positional arg for no-body operation to fail")
	}
	if !strings.Contains(err.Error(), "too many arguments: expected 1, got 2") {
		t.Fatalf("expected extra argument error, got: %v", err)
	}
}

func TestGeneratedCommandProfileBaseURLUsesSharedSpec(t *testing.T) {
	var gotHost string
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		gotHost = r.Host
		w.WriteHeader(200)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	staging := httptest.NewServer(mux)
	t.Cleanup(staging.Close)
	cfg, err := config.Load(env.cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.APIs["tapi"].Profiles = map[string]*config.ProfileConfig{
		"staging": {BaseURL: staging.URL},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(env.cfgFile, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c := env.newCLI()
	if err := c.Run([]string{"restish", "--rsh-profile", "staging", "tapi", "list-items"}); err != nil {
		t.Fatalf("list-items: %v", err)
	}
	if wantHost := strings.TrimPrefix(staging.URL, "http://"); gotHost != wantHost {
		t.Fatalf("request host = %q, want staging host %q", gotHost, wantHost)
	}
}

func TestGeneratedCommandPercentEscapesPathAndQueryArgs(t *testing.T) {
	var gotRequestURI string
	mux := http.NewServeMux()
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.WriteHeader(200)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	specBody := fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Percent API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/users/{email}": {
      "get": {
        "operationId": "getUser",
        "parameters": [
          {"name": "email", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "filter", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, srv.URL)
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specBody)
	})

	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{"pct": {BaseURL: srv.URL}}})
	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatal(err)
	}
	cacheDir := t.TempDir()
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "api", "sync", "pct"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}

	c = cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "pct", "get-user", "%@domain.com/a?b#c", "--filter", "%@domain.com"}); err != nil {
		t.Fatalf("get-user: %v", err)
	}
	if !strings.Contains(gotRequestURI, "/users/%25@domain.com%2Fa%3Fb%23c") {
		t.Fatalf("RequestURI = %q, want escaped generic syntax in path", gotRequestURI)
	}
	if !strings.Contains(gotRequestURI, "filter=%25%40domain.com") {
		t.Fatalf("RequestURI = %q, want escaped percent and at in query", gotRequestURI)
	}
}
