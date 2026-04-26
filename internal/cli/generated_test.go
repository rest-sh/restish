package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
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
						Auth: &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "secret"}},
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

func TestGeneratedCommandRequiredHeaderIsRequiredFlag(t *testing.T) {
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
		t.Fatal("expected missing required header flag to error")
	}

	c2 := env.newCLI()
	if err := c2.Run([]string{"restish", "tapi", "get-secure", "--x-auth", "secret"}); err != nil {
		t.Fatalf("get-secure with required header failed: %v", err)
	}
	if authHeader != "secret" {
		t.Fatalf("expected X-Auth header to be sent, got %q", authHeader)
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
	if !strings.Contains(err.Error(), "accepts 1 arg(s), received 2") {
		t.Fatalf("expected ExactArgs error, got: %v", err)
	}
}
