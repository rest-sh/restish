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
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/rest-sh/restish/v2/internal/auth"
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
              "note": {"type": ["string", "null"]},
              "count": {"type": "integer"},
              "tags": {
                "anyOf": [
                  {"type": "array", "items": {"type": "string"}},
                  {"type": "string"}
                ]
              },
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
    "/optional-body": {
      "post": {
        "operationId": "optionalBody",
        "summary": "Accept an optional body",
        "requestBody": {
          "required": false,
          "content": {"application/json": {"schema": {"type": "object"}}}
        },
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

func specWithMultiBodyOperation(baseURL string) string {
	return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Multi Body API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/multi-body": {
      "post": {
        "operationId": "multiBody",
        "summary": "Accept multiple request body media types",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {"schema": {
              "type": "object",
              "properties": {"jsonOnly": {"type": "string"}}
            }},
            "application/x-www-form-urlencoded": {"schema": {
              "type": "object",
              "properties": {"formOnly": {"type": "string"}}
            }},
            "text/plain": {
              "schema": {"type": "string"},
              "example": "hello generated text"
            }
          }
        },
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
}

func specWithAcceptOperation(baseURL string) string {
	return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Accept API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/negotiated": {
      "get": {
        "operationId": "getNegotiated",
        "responses": {
          "200": {
            "description": "OK",
            "content": {
              "application/vnd.example+json": {"schema": {"type": "object"}},
              "application/json": {"schema": {"type": "object"}},
              "application/cbor": {"schema": {"type": "object"}},
              "text/plain": {"schema": {"type": "string"}},
              "application/x-unknown": {"schema": {"type": "string"}}
            }
          }
        }
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
	return setupGeneratedEnvWithSpec(t, srv, mux, spec)
}

func setupGeneratedEnvWithSpec(t *testing.T, srv *httptest.Server, mux *http.ServeMux, spec string) *generatedEnv {
	t.Helper()

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
	_ = os.WriteFile(cfgFile, cfgData, 0o600)
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

func setupGeneratedEnvForSpec(t *testing.T, mux *http.ServeMux, specForBaseURL func(string) string) *generatedEnv {
	t.Helper()

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return setupGeneratedEnvWithSpec(t, srv, mux, specForBaseURL(srv.URL))
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

func expireGeneratedSpecCache(t *testing.T, cacheDir, apiName string) {
	t.Helper()
	path := filepath.Join(cacheDir, apiName+".cbor")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read spec cache: %v", err)
	}
	var entry map[string]any
	if err := cbor.Unmarshal(data, &entry); err != nil {
		t.Fatalf("decode spec cache: %v", err)
	}
	entry["fetched_at"] = time.Now().Add(-48 * time.Hour)
	entry["expires_at"] = time.Now().Add(-24 * time.Hour)
	data, err = cbor.Marshal(entry)
	if err != nil {
		t.Fatalf("encode spec cache: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write spec cache: %v", err)
	}
}

type countingLoader struct {
	detects atomic.Int32
}

func (l *countingLoader) Detect(contentType string, body []byte) bool {
	l.detects.Add(1)
	return false
}

func (l *countingLoader) LoadWithOptions(body []byte, _ spec.LoadOptions) (*spec.APISpec, error) {
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

func TestGeneratedAPIHelpTruncatesLongDescriptionAndHelpAllShowsFull(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	var descLines []string
	for i := 1; i <= 20; i++ {
		descLines = append(descLines, fmt.Sprintf("Line %02d: generated API description detail.", i))
	}
	descLines = append(descLines, "FULL DESCRIPTION SENTINEL")
	description := strings.Join(descLines, "\n")

	env := setupGeneratedEnvForSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Long Help API", "version": "1.0", "description": %q},
  "servers": [{"url": %q}],
  "paths": {
    "/items": {"get": {"operationId": "listItems", "responses": {"200": {"description": "OK"}}}}
  }
}`, description, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("tapi --help: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Line 01: generated API description detail.") {
		t.Fatalf("expected leading API description in help, got:\n%s", got)
	}
	if strings.Contains(got, "FULL DESCRIPTION SENTINEL") {
		t.Fatalf("expected default help to truncate full API description, got:\n%s", got)
	}
	if !strings.Contains(got, `Description truncated; run "restish tapi --help-all" to show the full API description.`) {
		t.Fatalf("expected truncation note in help, got:\n%s", got)
	}
	if !strings.Contains(got, "list-items") {
		t.Fatalf("expected generated command list after truncated description, got:\n%s", got)
	}

	c, out = env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "--help-all", "--help"}); err != nil {
		t.Fatalf("tapi --help-all --help: %v", err)
	}
	got = out.String()
	if !strings.Contains(got, "FULL DESCRIPTION SENTINEL") {
		t.Fatalf("expected help-all to show full API description, got:\n%s", got)
	}
	if strings.Contains(got, "Description truncated;") {
		t.Fatalf("did not expect truncation note in help-all output, got:\n%s", got)
	}
}

func TestGeneratedAPIHelpUsesStaleOperationCache(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnv(t, mux)
	expireGeneratedSpecCache(t, env.cacheDir, "tapi")
	c, out := env.newCaptureCLI()
	loader := &countingLoader{}
	c.AddLoader(loader)

	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("tapi --help: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "list-items") {
		t.Fatalf("expected stale generated operations in help, got:\n%s", got)
	}
	if got := loader.detects.Load(); got != 0 {
		t.Fatalf("loader Detect called %d times, want 0 when API help loads from stale cached operations", got)
	}
}

func TestGeneratedCommandRefreshesStaleOperationCacheOnUse(t *testing.T) {
	var specHits atomic.Int32
	mux := http.NewServeMux()
	var serverURL string
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		specHits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specWithOperations(serverURL))
	})
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	serverURL = srv.URL

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	env := &generatedEnv{cfgFile: cfgFile, cacheDir: t.TempDir()}
	syncCLI := env.newCLI()
	if err := syncCLI.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}
	hitsAfterSync := specHits.Load()
	expireGeneratedSpecCache(t, env.cacheDir, "tapi")

	c := env.newCLI()
	var out strings.Builder
	c.Stdout = &out
	if err := c.Run([]string{"restish", "tapi", "list-items"}); err != nil {
		t.Fatalf("generated command from stale cache: %v", err)
	}
	if got := specHits.Load(); got <= hitsAfterSync {
		t.Fatalf("expected stale generated command to refresh spec metadata; spec hits before=%d after=%d", hitsAfterSync, got)
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
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
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

func TestGeneratedCommandVerboseRedactsUncommonAPIKeyHeader(t *testing.T) {
	var gotHeader string
	mux := http.NewServeMux()
	mux.HandleFunc("/protected", func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Environment-Key")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "security": [{"EnvironmentKey": []}],
  "components": {
    "securitySchemes": {
      "EnvironmentKey": {"type": "apiKey", "in": "header", "name": "X-Environment-Key"}
    }
  },
  "paths": {
    "/protected": {
      "get": {
        "operationId": "getProtected",
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
						Credentials: map[string]*config.CredentialConfig{
							"EnvironmentKey": {
								Auth: &config.AuthConfig{
									Type:   "api-key",
									Params: map[string]string{"in": "header", "name": "X-Environment-Key", "value": "dummy"},
								},
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

	c, output := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-protected", "-v"}); err != nil {
		t.Fatalf("get-protected failed: %v", err)
	}
	if gotHeader != "dummy" {
		t.Fatalf("X-Environment-Key = %q, want configured credential", gotHeader)
	}
	stderr := output.String()
	if strings.Contains(stderr, "dummy") {
		t.Fatalf("verbose output leaked generated API-key header:\n%s", stderr)
	}
	if !strings.Contains(stderr, "> X-Environment-Key: <redacted>") {
		t.Fatalf("expected generated API-key header redacted, got:\n%s", stderr)
	}
}

func TestGeneratedCommandVerboseRedactsUncommonAPIKeyQuery(t *testing.T) {
	var gotQuery url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Query Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "security": [{"QuerystringAuthentication": []}],
  "components": {
    "securitySchemes": {
      "QuerystringAuthentication": {"type": "apiKey", "in": "query", "name": "u=&p="}
    }
  },
  "paths": {
    "/health": {
      "get": {
        "operationId": "getHealth",
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
						Credentials: map[string]*config.CredentialConfig{
							"QuerystringAuthentication": {
								Auth: &config.AuthConfig{
									Type:   "api-key",
									Params: map[string]string{"in": "query", "name": "u=&p=", "value": "dummy-token"},
								},
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

	c, output := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "get-health", "-v"}); err != nil {
		t.Fatalf("get-health failed: %v", err)
	}
	if got := gotQuery.Get("u=&p="); got != "dummy-token" {
		t.Fatalf("u=&p= query credential = %q, want dummy-token", got)
	}
	stderr := output.String()
	if strings.Contains(stderr, "dummy-token") {
		t.Fatalf("verbose output leaked generated query API key:\n%s", stderr)
	}
	if !strings.Contains(stderr, "u%3D%26p%3D=%3Credacted%3E") {
		t.Fatalf("expected generated query API key redacted, got:\n%s", stderr)
	}
}

func TestGeneratedCommandUserAgentAPIKeyOverridesDefaultUserAgent(t *testing.T) {
	var gotUserAgents []string
	mux := http.NewServeMux()
	mux.HandleFunc("/alerts", func(w http.ResponseWriter, r *http.Request) {
		gotUserAgents = append(gotUserAgents, r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Weather API", "version": "1.0"},
  "servers": [{"url": %q}],
  "security": [{"userAgent": []}],
  "components": {
    "securitySchemes": {
      "userAgent": {"type": "apiKey", "in": "header", "name": "User-Agent"}
    }
  },
  "paths": {
    "/alerts": {
      "get": {
        "operationId": "getAlerts",
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
						Credentials: map[string]*config.CredentialConfig{
							"userAgent": {
								Auth: &config.AuthConfig{
									Type:   "api-key",
									Params: map[string]string{"in": "header", "name": "User-Agent", "value": "restish-test-contact-example"},
								},
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
	if err := c.Run([]string{"restish", "tapi", "get-alerts"}); err != nil {
		t.Fatalf("get-alerts failed: %v", err)
	}
	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-alerts", "--rsh-header", "User-Agent: explicit-header-test"}); err != nil {
		t.Fatalf("get-alerts with explicit User-Agent failed: %v", err)
	}

	if len(gotUserAgents) != 2 {
		t.Fatalf("User-Agent requests = %v, want 2 requests", gotUserAgents)
	}
	if gotUserAgents[0] != "restish-test-contact-example" {
		t.Fatalf("configured User-Agent auth = %q, want contact example", gotUserAgents[0])
	}
	if gotUserAgents[1] != "explicit-header-test" {
		t.Fatalf("explicit User-Agent = %q, want explicit override", gotUserAgents[1])
	}
}

func TestGenericURLAppliesMatchedOperationAuth(t *testing.T) {
	var got []string
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/basic", func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Header.Get("Authorization"))
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
      "BasicAuth": {"type": "http", "scheme": "basic"}
    }
  },
  "paths": {
    "/auth/basic": {
      "get": {
        "operationId": "getAuthBasic",
        "security": [{"BasicAuth": []}],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})
	baseURL := strings.TrimSpace(readBaseURLFromConfig(t, env.cfgFile))
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: baseURL,
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Credentials: map[string]*config.CredentialConfig{
							"BasicAuth": {
								Auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": "alice", "password": "secret"}},
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
	if err := c.Run([]string{"restish", baseURL + "/auth/basic"}); err != nil {
		t.Fatalf("full URL request failed: %v", err)
	}
	c = env.newCLI()
	if err := c.Run([]string{"restish", strings.TrimPrefix(baseURL, "http://") + "/auth/basic"}); err != nil {
		t.Fatalf("scheme-less URL request failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("requests = %d, want 2", len(got))
	}
	for i, auth := range got {
		if !strings.HasPrefix(auth, "Basic ") {
			t.Fatalf("request %d Authorization = %q, want Basic auth", i+1, auth)
		}
	}
}

func TestGenericURLMatchedSecurityEmptySuppressesProfileAuth(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/public", func(w http.ResponseWriter, r *http.Request) {
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
      "BasicAuth": {"type": "http", "scheme": "basic"}
    }
  },
  "security": [{"BasicAuth": []}],
  "paths": {
    "/public": {
      "get": {
        "operationId": "getPublic",
        "security": [],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})
	baseURL := strings.TrimSpace(readBaseURLFromConfig(t, env.cfgFile))
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: baseURL,
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
	if err := c.Run([]string{"restish", baseURL + "/public"}); err != nil {
		t.Fatalf("generic public request failed: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization = %q, want empty for matched security: [] URL", gotAuth)
	}
}

func TestGenericURLRouteMatchPrefersExactPathOverTemplate(t *testing.T) {
	var gotExactAuth, gotTemplateAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/things/me", func(w http.ResponseWriter, r *http.Request) {
		gotExactAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/things/42", func(w http.ResponseWriter, r *http.Request) {
		gotTemplateAuth = r.Header.Get("Authorization")
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
      "BasicAuth": {"type": "http", "scheme": "basic"}
    }
  },
  "security": [{"BasicAuth": []}],
  "paths": {
    "/things/{id}": {
      "get": {
        "operationId": "getThing",
        "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {"200": {"description": "OK"}}
      }
    },
    "/things/me": {
      "get": {
        "operationId": "getMe",
        "security": [],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})
	baseURL := strings.TrimSpace(readBaseURLFromConfig(t, env.cfgFile))
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: baseURL,
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
	if err := c.Run([]string{"restish", baseURL + "/things/me"}); err != nil {
		t.Fatalf("generic exact route request failed: %v", err)
	}
	if gotExactAuth != "" {
		t.Fatalf("Authorization = %q, want exact security: [] route to win over templated secured route", gotExactAuth)
	}
	c = env.newCLI()
	if err := c.Run([]string{"restish", baseURL + "/things/42"}); err != nil {
		t.Fatalf("generic templated route request failed: %v", err)
	}
	if !strings.HasPrefix(gotTemplateAuth, "Basic ") {
		t.Fatalf("templated route Authorization = %q, want Basic auth", gotTemplateAuth)
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

func TestGeneratedCommandOAuthAuthorizationCodeRequiresCachedTokenBeforeSending(t *testing.T) {
	var hits atomic.Int32
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "OAuth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "OAuth": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {
            "authorizationUrl": "https://auth.example.com/authorize",
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {"read:profile": "Read profile"}
          }
        }
      }
    }
  },
  "paths": {
    "/profile": {"get": {"operationId": "getProfile", "security": [{"OAuth": ["read:profile"]}], "responses": {"200": {"description": "OK"}}}}
  }
}`, baseURL)
	})
	baseURL := readBaseURLFromConfig(t, env.cfgFile)
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {
				BaseURL: baseURL,
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Credentials: map[string]*config.CredentialConfig{
							"OAuth": {
								Auth: &config.AuthConfig{Type: "oauth-authorization-code", Params: map[string]string{
									"client_id":     "client",
									"authorize_url": "https://auth.example.com/authorize",
									"token_url":     "https://auth.example.com/token",
									"scopes":        "read:profile",
								}},
								Satisfies: []string{"read:profile"},
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
	tokenPath := filepath.Join(t.TempDir(), "tokens.cbor")

	c := env.newCLI()
	c.Hooks().TokenCachePath = tokenPath
	err := c.Run([]string{"restish", "tapi", "get-profile"})
	if err == nil {
		t.Fatal("expected missing cached OAuth token error")
	}
	if !strings.Contains(err.Error(), "no cached access token") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("request hits = %d, want 0 before OAuth token is available", got)
	}

	if err := auth.NewTokenCache(tokenPath).Set("tapi:default:credential:OAuth", auth.CachedToken{
		AccessToken: "cached-token",
		Expiry:      time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("write token cache: %v", err)
	}
	c = env.newCLI()
	c.Hooks().TokenCachePath = tokenPath
	if err := c.Run([]string{"restish", "tapi", "get-profile"}); err != nil {
		t.Fatalf("generated command with cached token: %v", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("request hits = %d, want 1 with cached OAuth token", got)
	}
	if gotAuth != "Bearer cached-token" {
		t.Fatalf("Authorization = %q, want Bearer cached-token", gotAuth)
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
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
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
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
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

func TestGeneratedCommandOperatorFlagNamesPreserveWireQueryNames(t *testing.T) {
	var gotQuery url.Values
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	specDoc := fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Operator Params", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/events": {
      "get": {
        "operationId": "listEvents",
        "parameters": [
          {"name": "StartTime", "in": "query", "schema": {"type": "string"}},
          {"name": "StartTime<", "in": "query", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, srv.URL)

	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specDoc)
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL},
		},
	})
	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	env := &generatedEnv{cfgFile: cfgFile, cacheDir: t.TempDir()}

	syncCLI := env.newCLI()
	if err := syncCLI.Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-events", "--start-time", "2024-01-01", "--start-time-lt", "2024-02-01"}); err != nil {
		t.Fatalf("list-events with operator flags: %v", err)
	}
	if got := gotQuery.Get("StartTime"); got != "2024-01-01" {
		t.Fatalf("StartTime query = %q, want 2024-01-01 (raw %#v)", got, gotQuery)
	}
	if got := gotQuery.Get("StartTime<"); got != "2024-02-01" {
		t.Fatalf("StartTime< query = %q, want 2024-02-01 (raw %#v)", got, gotQuery)
	}
}

func TestGeneratedCommandEnumTypeMismatchUsesStringFlag(t *testing.T) {
	var gotQuery url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Enum API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/events": {
      "get": {
        "operationId": "listEvents",
        "parameters": [
          {"name": "magID", "in": "query", "schema": {"type": "integer", "enum": ["mag_kts", "mms", "sq_NM"]}},
          {"name": "limit", "in": "query", "schema": {"type": "integer"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "list-events", "--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	help := out.String()
	if !strings.Contains(help, "--mag-id") || !strings.Contains(help, "enum:mag_kts,mms,sq_NM") {
		t.Fatalf("help missing enum detail:\n%s", help)
	}
	if !strings.Contains(help, "--mag-id: (string enum:mag_kts,mms,sq_NM)") {
		t.Fatalf("help schema should use effective string type:\n%s", help)
	}
	if strings.Contains(help, "--mag-id: (integer enum:mag_kts,mms,sq_NM)") {
		t.Fatalf("help schema should not keep incompatible source integer type:\n%s", help)
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-events", "--mag-id", "mag_kts", "--limit", "1"}); err != nil {
		t.Fatalf("list-events with string enum value failed: %v", err)
	}
	if got := gotQuery.Get("magID"); got != "mag_kts" {
		t.Fatalf("magID query = %q, want mag_kts", got)
	}
	if got := gotQuery.Get("limit"); got != "1" {
		t.Fatalf("limit query = %q, want 1", got)
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

func TestGeneratedCommandRequiredBodyMissingFailsBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	err := c.Run([]string{"restish", "tapi", "create-item"})
	if err == nil {
		t.Fatal("expected missing required body error")
	}
	if !strings.Contains(err.Error(), "request body is required") {
		t.Fatalf("expected required body error, got %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("server was called %d times despite missing body", got)
	}
}

func TestGeneratedCommandRequiredBodyCanComeFromStdin(t *testing.T) {
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotBody, _ = io.ReadAll(r.Body)
		}
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	c.Stdin = strings.NewReader(`{"name":"Widget"}`)
	if err := c.Run([]string{"restish", "tapi", "create-item"}); err != nil {
		t.Fatalf("create-item with stdin body: %v", err)
	}
	if !bytes.Contains(gotBody, []byte(`"name":"Widget"`)) {
		t.Fatalf("expected stdin request body, got %s", gotBody)
	}
}

func TestGeneratedCommandOptionalBodyCanBeOmitted(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/optional-body", func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "optional-body"}); err != nil {
		t.Fatalf("optional-body without body: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("server calls = %d, want 1", got)
	}
}

func TestGeneratedCommandBodyPreservesShorthandTypes(t *testing.T) {
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
		"amount:", "42,",
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
		if _, ok := body[key].(float64); !ok {
			t.Fatalf("%s = %#v (%T), want JSON number from shorthand", key, body[key], body[key])
		}
	}
	if body["id"] != float64(123) {
		t.Fatalf("id = %#v, want number 123", body["id"])
	}
	if body["amount"] != float64(42) {
		t.Fatalf("amount = %#v, want number 42", body["amount"])
	}
	meta, ok := body["meta"].(map[string]any)
	if !ok {
		t.Fatalf("meta = %T, want object", body["meta"])
	}
	if meta["code"] != float64(456) {
		t.Fatalf("meta.code = %#v, want number 456", meta["code"])
	}
	if _, ok := body["count"].(float64); !ok {
		t.Fatalf("count = %#v (%T), want JSON number", body["count"], body["count"])
	}
	if _, ok := body["unknown"].(float64); !ok {
		t.Fatalf("unknown = %#v (%T), want unknown fields left as parsed numbers", body["unknown"], body["unknown"])
	}
}

func TestGeneratedCommandBodyPreservesNull(t *testing.T) {
	var gotBodies [][]byte
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			gotBodies = append(gotBodies, body)
		}
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "create-item", "name:", "Ada,", "note:", "null"}); err != nil {
		t.Fatalf("create-item shorthand null: %v", err)
	}
	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "create-item", `{"name":"Ada","note":null}`}); err != nil {
		t.Fatalf("create-item raw JSON null: %v", err)
	}

	if len(gotBodies) != 2 {
		t.Fatalf("got %d request bodies, want 2", len(gotBodies))
	}
	for i, gotBody := range gotBodies {
		var body map[string]any
		if err := json.Unmarshal(gotBody, &body); err != nil {
			t.Fatalf("body %d not valid JSON: %v — body: %s", i, err, gotBody)
		}
		if _, ok := body["note"]; !ok {
			t.Fatalf("body %d missing note: %#v", i, body)
		}
		if body["note"] != nil {
			t.Fatalf("body %d note = %#v (%T), want JSON null; body: %s", i, body["note"], body["note"], gotBody)
		}
	}
}

func TestGeneratedCommandAnyOfBodyPreservesArrayValues(t *testing.T) {
	var gotBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotBody, _ = io.ReadAll(r.Body)
		}
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	env := setupGeneratedEnv(t, mux)
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "create-item", "tags[]: alpha, tags[]: beta"}); err != nil {
		t.Fatalf("create-item anyOf array: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(gotBody, &body); err != nil {
		t.Fatalf("body not valid JSON: %v — body: %s", err, gotBody)
	}
	tags, ok := body["tags"].([]any)
	if !ok {
		t.Fatalf("tags = %#v (%T), want array", body["tags"], body["tags"])
	}
	if !reflect.DeepEqual(tags, []any{"alpha", "beta"}) {
		t.Fatalf("tags = %#v, want [alpha beta]", tags)
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

func TestGeneratedCommandGenerateBodyNormalizesScalarExamples(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/probe", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("generate body should not send a request")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Body Types", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/probe": {
      "post": {
        "operationId": "createProbe",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "enabled": {"type": "boolean", "default": "false"},
                  "count": {"type": "integer", "example": "7"},
                  "ratio": {"type": "number", "examples": ["1.5"]},
                  "tags": {"type": "array", "items": {"type": "integer"}, "example": ["1", "2"]},
                  "metadata": {
                    "type": "object",
                    "additionalProperties": {"type": "boolean"},
                    "example": {"flag": "true"}
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

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "create-probe", "--rsh-generate-body"}); err != nil {
		t.Fatalf("generate body: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(out.String()), &body); err != nil {
		t.Fatalf("generated body should be JSON: %v\n%s", err, out.String())
	}
	want := map[string]any{
		"enabled":  false,
		"count":    float64(7),
		"ratio":    1.5,
		"tags":     []any{float64(1), float64(2)},
		"metadata": map[string]any{"flag": true},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("generated body = %#v, want %#v\nraw:\n%s", body, want, out.String())
	}
	if strings.Contains(out.String(), `"false"`) || strings.Contains(out.String(), `"7"`) || strings.Contains(out.String(), `"1.5"`) {
		t.Fatalf("generated body should not quote typed scalar examples:\n%s", out.String())
	}
}

func TestGeneratedCommandGenerateBodyIgnoresConflictingTypedAnnotations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mixed", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("generate body should not send a request")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Mixed", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/mixed": {
      "post": {
        "operationId": "mixedEnum",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "id": {"type": "integer"}
                },
                "const": [null, "test", 1, ["nested", false]],
                "enum": [[null, "test", 1, ["nested", false]]]
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

	c, out := env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "mixed-enum", "--rsh-generate-body"}); err != nil {
		t.Fatalf("generate body: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(out.String()), &body); err != nil {
		t.Fatalf("generated body should fall back to object shape: %v\n%s", err, out.String())
	}
	if !reflect.DeepEqual(body, map[string]any{"id": float64(1)}) {
		t.Fatalf("generated body = %#v, want object shape\nraw:\n%s", body, out.String())
	}
	if strings.Contains(out.String(), `"nested"`) {
		t.Fatalf("generated body should ignore conflicting root enum/const:\n%s", out.String())
	}
}

func TestGeneratedCommandGenerateBodyHonorsContentType(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/multi-body", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("generate body should not send a request")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, specWithMultiBodyOperation)
	tests := []struct {
		name    string
		args    []string
		want    string
		notWant string
	}{
		{
			name:    "default",
			args:    []string{"restish", "tapi", "multi-body", "--rsh-generate-body"},
			want:    `"jsonOnly": "string"`,
			notWant: "formOnly",
		},
		{
			name:    "form",
			args:    []string{"restish", "tapi", "multi-body", "--rsh-content-type", "application/x-www-form-urlencoded", "--rsh-generate-body"},
			want:    `"formOnly": "string"`,
			notWant: "jsonOnly",
		},
		{
			name:    "form alias",
			args:    []string{"restish", "tapi", "multi-body", "--rsh-content-type", "form", "--rsh-generate-body"},
			want:    `"formOnly": "string"`,
			notWant: "jsonOnly",
		},
		{
			name:    "text",
			args:    []string{"restish", "tapi", "multi-body", "--rsh-content-type", "text/plain", "--rsh-generate-body"},
			want:    `"hello generated text"`,
			notWant: "jsonOnly",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, out := env.newCaptureCLI()
			if err := c.Run(tc.args); err != nil {
				t.Fatalf("generate body: %v", err)
			}
			got := out.String()
			if !strings.Contains(got, tc.want) {
				t.Fatalf("generated body missing %q:\n%s", tc.want, got)
			}
			if strings.Contains(got, tc.notWant) {
				t.Fatalf("generated body should not contain %q:\n%s", tc.notWant, got)
			}
		})
	}
}

func TestGeneratedCommandGenerateBodyRejectsUndeclaredContentType(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, specWithMultiBodyOperation)
	c := env.newCLI()
	err := c.Run([]string{"restish", "tapi", "multi-body", "--rsh-content-type", "application/xml", "--rsh-generate-body"})
	if err == nil {
		t.Fatal("expected undeclared content type error")
	}
	if !strings.Contains(err.Error(), `request body content type "application/xml" is not declared`) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "application/json") || !strings.Contains(err.Error(), "application/x-www-form-urlencoded") || !strings.Contains(err.Error(), "text/plain") {
		t.Fatalf("error should list available content types: %v", err)
	}
}

func TestGeneratedCommandAcceptHeaderIncludesDeclaredSupportedResponseTypes(t *testing.T) {
	var gotAccept string
	mux := http.NewServeMux()
	mux.HandleFunc("/negotiated", func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupGeneratedEnvForSpec(t, mux, specWithAcceptOperation)
	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-negotiated"}); err != nil {
		t.Fatalf("get-negotiated: %v", err)
	}
	want := "application/cbor;q=0.9, application/vnd.example+json;q=0.5, application/json;q=0.5, text/plain;q=0.2"
	if gotAccept != want {
		t.Fatalf("Accept = %q, want %q", gotAccept, want)
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-negotiated", "-H", "Accept: text/plain"}); err != nil {
		t.Fatalf("get-negotiated override: %v", err)
	}
	if gotAccept != "text/plain" {
		t.Fatalf("overridden Accept = %q, want text/plain", gotAccept)
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
	if err := c.Run([]string{"restish", "tapi", "--help"}); err != nil {
		t.Fatalf("api help: %v", err)
	}
	apiHelp := out.String()
	if !strings.Contains(apiHelp, "--help-all") {
		t.Fatalf("generated API help should point to --help-all, got:\n%s", apiHelp)
	}

	c, out = env.newCaptureCLI()
	if err := c.Run([]string{"restish", "tapi", "--help-all", "--help"}); err != nil {
		t.Fatalf("api help-all: %v", err)
	}
	apiHelpAll := out.String()
	if !strings.Contains(apiHelpAll, "--rsh-auth") || !strings.Contains(apiHelpAll, "--rsh-config") {
		t.Fatalf("generated API help-all should include auth and config flags, got:\n%s", apiHelpAll)
	}

	c, out = env.newCaptureCLI()
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
	authGroupIdx := strings.Index(full, "Auth and Profile Options")
	authFlagIdx := strings.Index(full, "--rsh-auth")
	configGroupIdx := strings.Index(full, "General Options")
	configFlagIdx := strings.Index(full, "--rsh-config")
	if authGroupIdx < 0 || authFlagIdx < authGroupIdx {
		t.Fatalf("help-all should group --rsh-auth under auth options, got:\n%s", full)
	}
	if configGroupIdx < 0 || configFlagIdx < configGroupIdx {
		t.Fatalf("help-all should group --rsh-config under general options, got:\n%s", full)
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

	c, out = env.newCaptureCLI()
	c.SetCommandName("myapp")
	if err := c.Run([]string{"myapp", "tapi", "create-pet", "--help"}); err != nil {
		t.Fatalf("help with custom command name: %v", err)
	}
	got = out.String()
	if !strings.Contains(got, "myapp tapi create-pet name: Fluffy, secret_token: abc123") {
		t.Fatalf("expected generated example to use custom command name, got:\n%s", got)
	}
	if strings.Contains(got, "restish tapi create-pet name: Fluffy") {
		t.Fatalf("generated example used hard-coded command name, got:\n%s", got)
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
          "loginStatus": {
            "type": "integer",
            "oneOf": [
              {"const": 0, "description": "Failure"},
              {"const": 1, "description": "Success"},
              {"const": 2, "description": "Pending"}
            ]
          },
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
		"(integer const:0) Failure",
		"(integer const:1) Success",
		"(integer const:2) Pending",
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
	if strings.Contains(got, "(string const:0)") {
		t.Fatalf("numeric const branches should not render as strings, got:\n%s", got)
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
          {"name": "region", "in": "query", "style": "form", "explode": true, "schema": {"type": "array", "items": {"type": "string"}, "default": ["us-east-1,blue", "green"]}},
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
	if got := values.Get("limit"); got != "" {
		t.Fatalf("limit default = %q, want omitted", got)
	}
	if got := values.Get("score"); got != "1.5" {
		t.Fatalf("score = %q, want 1.5", got)
	}
	if got := values["tag"]; strings.Join(got, ",") != "red,blue" {
		t.Fatalf("tag values = %v, want red and blue", got)
	}
	if got := values["region"]; got != nil {
		t.Fatalf("region values = %v, want omitted default", got)
	}
	if got := values.Get("ids"); got != "1,2" {
		t.Fatalf("ids = %q, want 1,2", got)
	}
}

func TestGeneratedCommandReservedFlagCollisionsAreRenamed(t *testing.T) {
	var gotQuery string
	var gotHeader string
	mux := http.NewServeMux()
	mux.HandleFunc("/collide", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotHeader = r.Header.Get("X-Help")
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
    "/collide": {
      "get": {
        "operationId": "getCollision",
        "parameters": [
          {"name": "help", "in": "query", "schema": {"type": "string"}},
          {"name": "rsh-output-format", "in": "query", "schema": {"type": "string"}},
          {"name": "foo-bar", "in": "query", "schema": {"type": "string"}},
          {"name": "foo_bar", "in": "query", "schema": {"type": "string"}},
          {"name": "X-Help", "in": "header", "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	var helpOut bytes.Buffer
	c := env.newCLI()
	c.Stdout = &helpOut
	if err := c.Run([]string{"restish", "tapi", "get-collision", "--help"}); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	help := helpOut.String()
	for _, want := range []string{"--query-help", "--query-rsh-output-format", "--foo-bar", "--foo-underscore-bar", "--x-help"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %s:\n%s", want, help)
		}
	}
	if strings.Contains(help, "--foo-bar-foo-underscore-bar") {
		t.Fatalf("help included duplicated-base flag:\n%s", help)
	}
	if strings.Contains(help, "--help string") || strings.Contains(help, "--rsh-output-format string") {
		t.Fatalf("help included shadowing generated flags:\n%s", help)
	}

	c = env.newCLI()
	if err := c.Run([]string{
		"restish", "tapi", "get-collision",
		"--query-help", "from-query",
		"--query-rsh-output-format", "from-query-format",
		"--foo-bar", "from-dash",
		"--foo-underscore-bar", "from-underscore",
		"--x-help", "from-header",
		"-o", "json",
	}); err != nil {
		t.Fatalf("get-collision failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("help"); got != "from-query" {
		t.Fatalf("help query = %q, want from-query", got)
	}
	if got := values.Get("rsh-output-format"); got != "from-query-format" {
		t.Fatalf("rsh-output-format query = %q, want from-query-format", got)
	}
	if got := values.Get("foo-bar"); got != "from-dash" {
		t.Fatalf("foo-bar query = %q, want from-dash", got)
	}
	if got := values.Get("foo_bar"); got != "from-underscore" {
		t.Fatalf("foo_bar query = %q, want from-underscore", got)
	}
	if gotHeader != "from-header" {
		t.Fatalf("X-Help header = %q, want from-header", gotHeader)
	}
}

func TestGeneratedCommandJSONContentQueryChildFlagsUseParentEncoding(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/jsoncontent", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
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
    "/jsoncontent": {
      "get": {
        "operationId": "getJSONContent",
        "parameters": [
          {"name": "filter", "in": "query", "content": {
            "application/json": {"schema": {
              "type": "object",
              "properties": {
                "tag": {"type": "string"},
                "limit": {"type": "integer"},
                "active": {"type": "boolean"}
              }
            }}
          }}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-json-content", "--filter-tag", "alpha", "--filter-limit", "7", "--filter-active"}); err != nil {
		t.Fatalf("get-json-content failed: %v", err)
	}
	if strings.Contains(gotQuery, "filter%5B") || strings.Contains(gotQuery, "filter[") {
		t.Fatalf("child flags used deep-object query shape: %q", gotQuery)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	var filter map[string]any
	if err := json.Unmarshal([]byte(values.Get("filter")), &filter); err != nil {
		t.Fatalf("filter is not JSON object: %v; raw query %q", err, gotQuery)
	}
	if filter["tag"] != "alpha" || filter["limit"] != float64(7) || filter["active"] != true {
		t.Fatalf("filter = %#v, want tag alpha, limit 7, active true", filter)
	}
}

func TestGeneratedCommandJSONContentArrayQueryFlagUsesJSONArray(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/jsoncontent", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
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
    "/jsoncontent": {
      "get": {
        "operationId": "getJSONContent",
        "parameters": [
          {"name": "ids", "in": "query", "content": {
            "application/json": {"schema": {"type": "array", "items": {"type": "string"}}}
          }},
          {"name": "mode", "in": "query", "content": {
            "application/json": {"schema": {"type": "string"}}
          }}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-json-content", "--ids", "a", "--ids", "b", "--mode", "fast"}); err != nil {
		t.Fatalf("get-json-content failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	var ids []string
	if err := json.Unmarshal([]byte(values.Get("ids")), &ids); err != nil {
		t.Fatalf("ids is not JSON array: %v; raw query %q", err, gotQuery)
	}
	if !reflect.DeepEqual(ids, []string{"a", "b"}) {
		t.Fatalf("ids = %#v, want [a b]", ids)
	}
	if mode := values.Get("mode"); mode != `"fast"` {
		t.Fatalf("mode = %q, want JSON string", mode)
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-json-content", "--ids", `["raw","json"]`}); err != nil {
		t.Fatalf("get-json-content raw JSON failed: %v", err)
	}
	values, err = url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("ids"); got != `["raw","json"]` {
		t.Fatalf("raw JSON ids = %q, want raw array", got)
	}
}

func TestGeneratedCommandOptionalDefaultsSentOnlyWhenChanged(t *testing.T) {
	var queries []string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos", func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "GitHub-ish API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/repos": {
      "get": {
        "operationId": "listRepos",
        "parameters": [
          {"name": "type", "in": "query", "schema": {"type": "string", "default": "all"}},
          {"name": "visibility", "in": "query", "schema": {"type": "string", "default": "all"}},
          {"name": "affiliation", "in": "query", "schema": {"type": "string", "default": "owner"}},
          {"name": "archived", "in": "query", "schema": {"type": "boolean", "default": true}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-repos"}); err != nil {
		t.Fatalf("list-repos failed: %v", err)
	}
	if queries[0] != "" {
		t.Fatalf("omitted optional defaults query = %q, want empty", queries[0])
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "list-repos", "--visibility", "", "--archived=false"}); err != nil {
		t.Fatalf("list-repos with explicit values failed: %v", err)
	}
	values, err := url.ParseQuery(queries[1])
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", queries[1], err)
	}
	if got, ok := values["visibility"]; !ok || len(got) != 1 || got[0] != "" {
		t.Fatalf("visibility = %#v, want explicit empty string", values["visibility"])
	}
	if got := values.Get("archived"); got != "false" {
		t.Fatalf("archived = %q, want false", got)
	}

	c = env.newCLI()
	out := &bytes.Buffer{}
	c.Stdout = out
	_ = c.Run([]string{"restish", "tapi", "list-repos", "--help"})
	if got := out.String(); !strings.Contains(got, "Default: all") || !strings.Contains(got, "Default: true") {
		t.Fatalf("help did not document server defaults:\n%s", got)
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

func TestGeneratedCommandRequiredQueryAPIKeySatisfiedByAuth(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/types/board", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })

	env := setupEnvWithSpec(t, mux, func(baseURL string) string {
		return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Trello-ish API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "ApiKey": {"type": "apiKey", "in": "query", "name": "key"},
      "ApiToken": {"type": "apiKey", "in": "query", "name": "token"}
    }
  },
  "paths": {
    "/types/{id}": {
      "get": {
        "operationId": "getTypesById",
        "security": [{"ApiKey": [], "ApiToken": []}],
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "string"}},
          {"name": "key", "in": "query", "required": true, "schema": {"type": "string"}},
          {"name": "token", "in": "query", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {
			BaseURL: readBaseURLFromConfig(t, env.cfgFile),
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					Credentials: map[string]*config.CredentialConfig{
						"ApiKey": {
							Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "query", "name": "key", "value": "configured-key"}},
						},
						"ApiToken": {
							Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "query", "name": "token", "value": "configured-token"}},
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
	if err := c.Run([]string{"restish", "tapi", "get-types-by-id", "--help"}); err != nil {
		t.Fatalf("get-types-by-id --help: %v", err)
	}
	help := out.String()
	if strings.Contains(help, "<key>") || strings.Contains(help, "<token>") {
		t.Fatalf("help should not expose auth query params as required args:\n%s", help)
	}
	if !strings.Contains(help, "get-types-by-id <id>") {
		t.Fatalf("help missing id-only usage:\n%s", help)
	}

	c = env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-types-by-id", "board"}); err != nil {
		t.Fatalf("get-types-by-id with configured query auth failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("key"); got != "configured-key" {
		t.Fatalf("key = %q, want configured-key", got)
	}
	if got := values.Get("token"); got != "configured-token" {
		t.Fatalf("token = %q, want configured-token", got)
	}
}

func TestGeneratedCommandRequiredNegativeNumberArgument(t *testing.T) {
	var gotQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/forecast", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
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
    "/v1/forecast": {
      "get": {
        "operationId": "get-v1-forecast",
        "parameters": [
          {"name": "latitude", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "longitude", "in": "query", "required": true, "schema": {"type": "number"}},
          {"name": "current_weather", "in": "query", "schema": {"type": "boolean"}}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, baseURL)
	})

	c := env.newCLI()
	if err := c.Run([]string{"restish", "tapi", "get-v1-forecast", "47.5301", "-122.0326", "--current-weather"}); err != nil {
		t.Fatalf("get-v1-forecast with negative longitude failed: %v", err)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("latitude"); got != "47.5301" {
		t.Fatalf("latitude = %q, want 47.5301", got)
	}
	if got := values.Get("longitude"); got != "-122.0326" {
		t.Fatalf("longitude = %q, want -122.0326", got)
	}
	if got := values.Get("current_weather"); got != "true" {
		t.Fatalf("current_weather = %q, want true", got)
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
	if strings.Contains(out.String(), "warning: command name collision") {
		t.Fatalf("unexpected repeated collision warning in help output, got:\n%s", out.String())
	}

	c, out = env.newCaptureCLI()
	if err := c.Run([]string{"restish", "api", "list"}); err != nil {
		t.Fatalf("api list failed: %v", err)
	}
	if strings.Contains(out.String(), "warning: command name collision") {
		t.Fatalf("unexpected generated warning for unrelated built-in command:\n%s", out.String())
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
		return `{
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

func TestAPISetOperationBaseRegeneratesGeneratedCommandsFromRawCache(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	specBody := fmt.Sprintf(`{
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
}`, srv.URL+"/api/v1")
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specBody)
	})
	mux.HandleFunc("/override/my-op", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/api/v1/my-op", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected regenerated operation_base path, got %s", r.URL.Path)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: srv.URL + "/api/v1", SpecURL: srv.URL + "/openapi.json"},
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
	if err := c.Run([]string{"restish", "api", "set", "tapi", `operation_base: "/override"`}); err != nil {
		t.Fatalf("api set operation_base: %v", err)
	}

	c = cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = io.Discard
	c.Stderr = io.Discard
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "tapi", "my-op"}); err != nil {
		t.Fatalf("my-op after api set operation_base: %v", err)
	}
	if lastPath != "/override/my-op" {
		t.Fatalf("generated command path = %q, want /override/my-op", lastPath)
	}
}

func TestGeneratedCommandsResolveProfileOperationBaseAtExecutionTime(t *testing.T) {
	var lastPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/my-op", func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[]`)
	})
	mux.HandleFunc("/profile/api/v2/my-op", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("profile operation_base should escape the profile base path, got %s", r.URL.Path)
	})
	mux.HandleFunc("/api/v2/my-op", func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("default operation base should not be baked into profile execution, got %s", r.URL.Path)
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
}`, baseURL+"/api/v2")
	})
	cfg, err := config.Load(env.cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	baseURL := cfg.APIs["tapi"].BaseURL
	cfg.APIs["tapi"].BaseURL = baseURL + "/api/v2"
	cfg.APIs["tapi"].Profiles = map[string]*config.ProfileConfig{
		"default": {},
		"staging": {
			BaseURL:       baseURL + "/profile/api/v2",
			OperationBase: "/",
		},
	}
	if err := config.Save(env.cfgFile, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	c := env.newCLI()
	if err := c.Run([]string{"restish", "--rsh-profile", "staging", "tapi", "my-op"}); err != nil {
		t.Fatalf("my-op failed: %v", err)
	}
	if lastPath != "/my-op" {
		t.Fatalf("expected profile operation_base path to resolve at execution time, got %q", lastPath)
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
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
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

func TestGeneratedCommandCrossOriginOperationServerRequiresAllow(t *testing.T) {
	var gotInferencePath string
	inference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInferencePath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	t.Cleanup(inference.Close)

	controlMux := http.NewServeMux()
	control := httptest.NewServer(controlMux)
	t.Cleanup(control.Close)
	specBody := fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Control API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/v1/models": {
      "get": {
        "operationId": "listModels",
        "servers": [{"url": %q}],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, control.URL, inference.URL)
	controlMux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specBody)
	})
	controlMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	cacheDir := t.TempDir()
	writeCfg := func(allowed []string) {
		t.Helper()
		cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
			"tapi": {BaseURL: control.URL, AllowedOperationOrigins: allowed},
		}})
		if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	newCLI := func() *cli.CLI {
		c := cli.New()
		c.Stdin = strings.NewReader("")
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		c.Hooks().ConfigPath = cfgFile
		c.Hooks().SpecCachePath = cacheDir
		return c
	}

	writeCfg(nil)
	if err := newCLI().Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}
	err := newCLI().Run([]string{"restish", "tapi", "list-models"})
	if err == nil || !strings.Contains(err.Error(), "allowed_operation_origins") {
		t.Fatalf("expected cross-origin operation server error, got %v", err)
	}
	if gotInferencePath != "" {
		t.Fatalf("unexpected inference request before allow: %q", gotInferencePath)
	}

	writeCfg([]string{inference.URL})
	if err := newCLI().Run([]string{"restish", "tapi", "list-models"}); err != nil {
		t.Fatalf("list-models with allowed origin: %v", err)
	}
	if gotInferencePath != "/v1/models" {
		t.Fatalf("inference path = %q, want /v1/models", gotInferencePath)
	}
}

func TestGeneratedCommandCrossOriginOperationServerAppliesOperationAuth(t *testing.T) {
	got := map[string]*http.Request{}
	operationServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got[r.URL.Path] = r.Clone(r.Context())
		got[r.URL.Path].Header = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	t.Cleanup(operationServer.Close)

	controlMux := http.NewServeMux()
	control := httptest.NewServer(controlMux)
	t.Cleanup(control.Close)
	specBody := fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Cross-Origin Auth API", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "BearerAuth": {"type": "http", "scheme": "bearer"},
      "BasicAuth": {"type": "http", "scheme": "basic"},
      "HeaderKey": {"type": "apiKey", "in": "header", "name": "X-API-Key"},
      "QueryKey": {"type": "apiKey", "in": "query", "name": "api_key"},
      "CookieKey": {"type": "apiKey", "in": "cookie", "name": "session"}
    }
  },
  "paths": {
    "/bearer": {"get": {"operationId": "getBearer", "servers": [{"url": %q}], "security": [{"BearerAuth": []}], "responses": {"200": {"description": "OK"}}}},
    "/basic": {"get": {"operationId": "getBasic", "servers": [{"url": %q}], "security": [{"BasicAuth": []}], "responses": {"200": {"description": "OK"}}}},
    "/header": {"get": {"operationId": "getHeader", "servers": [{"url": %q}], "security": [{"HeaderKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/query": {"get": {"operationId": "getQuery", "servers": [{"url": %q}], "security": [{"QueryKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/cookie": {"get": {"operationId": "getCookie", "servers": [{"url": %q}], "security": [{"CookieKey": []}], "responses": {"200": {"description": "OK"}}}}
  }
}`, control.URL, operationServer.URL, operationServer.URL, operationServer.URL, operationServer.URL, operationServer.URL)
	controlMux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specBody)
	})
	controlMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	cacheDir := t.TempDir()
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"tapi": {
			BaseURL:                 control.URL,
			AllowedOperationOrigins: []string{operationServer.URL},
			Profiles: map[string]*config.ProfileConfig{
				"default": {
					Credentials: map[string]*config.CredentialConfig{
						"BearerAuth": {Auth: &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "bearer-token"}}},
						"BasicAuth":  {Auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": "u", "password": "p"}}},
						"HeaderKey":  {Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-API-Key", "value": "header-token"}}},
						"QueryKey":   {Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "query", "name": "api_key", "value": "query-token"}}},
						"CookieKey":  {Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "cookie", "name": "session", "value": "cookie-token"}}},
					},
				},
			},
		},
	}})
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	newCLI := func() *cli.CLI {
		c := cli.New()
		c.Stdin = strings.NewReader("")
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		c.Hooks().ConfigPath = cfgFile
		c.Hooks().SpecCachePath = cacheDir
		return c
	}
	if err := newCLI().Run([]string{"restish", "api", "sync", "tapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}
	for _, command := range []string{"get-bearer", "get-basic", "get-header", "get-query", "get-cookie"} {
		if err := newCLI().Run([]string{"restish", "tapi", command}); err != nil {
			t.Fatalf("%s failed: %v", command, err)
		}
	}

	if got["/bearer"].Header.Get("Authorization") != "Bearer bearer-token" {
		t.Fatalf("/bearer Authorization = %q", got["/bearer"].Header.Get("Authorization"))
	}
	if got["/basic"].Header.Get("Authorization") != "Basic dTpw" {
		t.Fatalf("/basic Authorization = %q", got["/basic"].Header.Get("Authorization"))
	}
	if got["/header"].Header.Get("X-API-Key") != "header-token" {
		t.Fatalf("/header X-API-Key = %q", got["/header"].Header.Get("X-API-Key"))
	}
	if got["/query"].URL.Query().Get("api_key") != "query-token" {
		t.Fatalf("/query api_key = %q", got["/query"].URL.Query().Get("api_key"))
	}
	if cookie, err := got["/cookie"].Cookie("session"); err != nil || cookie.Value != "cookie-token" {
		t.Fatalf("/cookie session cookie = %#v, %v", cookie, err)
	}
}

func TestGeneratedStripeDeepObjectQueryFlags(t *testing.T) {
	var gotQueries []url.Values
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/v1/customers", func(w http.ResponseWriter, r *http.Request) {
		gotQueries = append(gotQueries, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})

	specPath := filepath.Join(t.TempDir(), "stripe.json")
	specBody := fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Stripe-ish", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/v1/customers": {
      "get": {
        "operationId": "listCustomers",
        "parameters": [
          {"name": "expand", "in": "query", "style": "deepObject", "explode": true, "schema": {"type": "array", "items": {"type": "string"}}},
          {"name": "created", "in": "query", "style": "deepObject", "explode": true, "schema": {
            "anyOf": [
              {"type": "integer"},
              {"type": "object", "properties": {
                "gt": {"type": "integer"},
                "gte": {"type": "integer"},
                "lt": {"type": "integer"},
                "lte": {"type": "integer"}
              }}
            ]
          }}
        ],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, srv.URL)
	if err := os.WriteFile(specPath, []byte(specBody), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	cfgData, _ := json.Marshal(&config.Config{APIs: map[string]*config.APIConfig{
		"stripe": {BaseURL: srv.URL, SpecFiles: []string{specPath}},
	}})
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	newCLI := func() *cli.CLI {
		c := cli.New()
		c.Stdin = strings.NewReader("")
		c.Stdout = io.Discard
		c.Stderr = io.Discard
		c.Hooks().ConfigPath = cfgFile
		c.Hooks().SpecCachePath = t.TempDir()
		return c
	}

	if err := newCLI().Run([]string{"restish", "stripe", "list-customers", "--expand", "data.customer", "--expand", "data.invoice", "--created-gt", "0", "--created-lte", "99", "--rsh-query", "limit=1"}); err != nil {
		t.Fatalf("list-customers child flags: %v", err)
	}
	q := gotQueries[len(gotQueries)-1]
	if got := q["expand[]"]; !reflect.DeepEqual(got, []string{"data.customer", "data.invoice"}) {
		t.Fatalf("expand[] = %#v", got)
	}
	if q.Get("created[gt]") != "0" || q.Get("created[lte]") != "99" || q.Get("limit") != "1" {
		t.Fatalf("query = %#v, want created child flags and rsh-query", q)
	}

	if err := newCLI().Run([]string{"restish", "stripe", "list-customers", "--created", "1700000000"}); err != nil {
		t.Fatalf("list-customers scalar created: %v", err)
	}
	if q := gotQueries[len(gotQueries)-1]; q.Get("created") != "1700000000" {
		t.Fatalf("created scalar query = %#v", q)
	}

	err := newCLI().Run([]string{"restish", "stripe", "list-customers", "--created", "1", "--created-gt", "0"})
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected scalar-plus-child conflict, got %v", err)
	}

	var help strings.Builder
	c := newCLI()
	c.Stdout = &help
	if err := c.Run([]string{"restish", "stripe", "list-customers", "--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"--created-gt", "--created-gte", "--created-lt", "--created-lte"} {
		if !strings.Contains(help.String(), want) {
			t.Fatalf("expected %s in help:\n%s", want, help.String())
		}
	}
}
