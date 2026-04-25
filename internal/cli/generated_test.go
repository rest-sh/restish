package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	if got := loader.detects.Load(); got != 1 {
		t.Fatalf("loader Detect called %d times, want 1 targeted API load", got)
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
