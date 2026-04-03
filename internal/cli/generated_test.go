package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/cli"
	"github.com/danielgtaylor/restish/v2/internal/config"
)

// specWithOperations returns an OpenAPI 3.1 spec JSON string.
func specWithOperations(baseURL string) string {
	return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Test API", "version": "1.0"},
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
          "content": {"application/json": {"schema": {"type": "object"}}}
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
	c.ConfigPath = e.cfgFile
	c.SpecCachePath = e.cacheDir
	c.RetryBaseDelay = 0
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
