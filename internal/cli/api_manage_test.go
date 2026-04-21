package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/config"
)

// specWithXCLIConfig returns an OpenAPI spec with x-cli-config pre-populating
// a default profile.
func specWithXCLIConfig(baseURL string) string {
	return fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "servers": [{"url": %q}],
  "x-cli-config": {
    "profiles": {
      "default": {
        "headers": ["Accept: application/json"],
        "auth": {"type": "bearer", "params": {"token": ""}}
      }
    }
  },
  "paths": {}
}`, baseURL)
}

// TestAPIConfigure verifies that "api configure" fetches the spec, reads
// x-cli-config, and writes a config file with the pre-populated fields.
func TestAPIConfigure(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"

	c, out, _ := newTestCLI()
	c.ConfigPath = cfgFile
	c.SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case "https://api.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specWithXCLIConfig("https://api.example.com"))),
				Request:    r,
			}, nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	if err := c.Run([]string{"restish", "api", "configure", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api configure: %v", err)
	}

	if !strings.Contains(out.String(), "myapi") {
		t.Errorf("expected confirmation message, got: %q", out.String())
	}

	// Load the written config and verify the fields.
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	api, ok := written.APIs["myapi"]
	if !ok {
		t.Fatal("expected myapi in config")
	}
	if api.BaseURL != "https://api.example.com" {
		t.Errorf("base_url: got %q, want %q", api.BaseURL, "https://api.example.com")
	}
	prof := api.Profiles["default"]
	if prof == nil {
		t.Fatal("expected default profile in config")
	}
	if prof.Auth == nil || prof.Auth.Type != "bearer" {
		t.Errorf("expected bearer auth in default profile, got: %+v", prof.Auth)
	}
	if len(prof.Headers) == 0 || !strings.Contains(prof.Headers[0], "application/json") {
		t.Errorf("expected Accept header in default profile, got: %v", prof.Headers)
	}
}

func TestAPIConfigureAllowCrossOriginSpec(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	c.SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Link": []string{`<https://spec.example.com/openapi.json>; rel="service-desc"`}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case "https://spec.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specWithXCLIConfig("https://api.example.com"))),
				Request:    r,
			}, nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	if err := c.Run([]string{"restish", "api", "configure", "myapi", "https://api.example.com", "--allow-cross-origin-spec"}); err != nil {
		t.Fatalf("api configure: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	api, ok := written.APIs["myapi"]
	if !ok {
		t.Fatal("expected myapi in config")
	}
	if !api.AllowCrossOriginSpec {
		t.Fatal("expected allow_cross_origin_spec to be persisted")
	}
}

func TestAPIConfigureForceRefreshesCachedSpec(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	c.SpecCachePath = t.TempDir()

	currentHeader := "Accept: application/json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case "https://api.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "servers": [{"url": "https://api.example.com"}],
  "x-cli-config": {
    "profiles": {
      "default": {
        "headers": [%q]
      }
    }
  },
  "paths": {}
}`, currentHeader))),
				Request: r,
			}, nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	if err := c.Run([]string{"restish", "api", "configure", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("first api configure: %v", err)
	}

	currentHeader = "Accept: application/vnd.api+json"
	if err := c.Run([]string{"restish", "api", "configure", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("second api configure: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if got := written.APIs["myapi"].Profiles["default"].Headers[0]; got != currentHeader {
		t.Fatalf("expected refreshed x-cli-config header %q, got %q", currentHeader, got)
	}
}

// TestAPIShow verifies that "api show" prints the API config as JSON.
func TestAPIShow(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o644)

	c, out, _ := newTestCLI()
	c.ConfigPath = cfgFile

	if err := c.Run([]string{"restish", "api", "show", "myapi"}); err != nil {
		t.Fatalf("api show: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "api.example.com") {
		t.Errorf("expected base_url in output, got: %q", got)
	}
	// Validate that output is valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &parsed); err != nil {
		t.Errorf("api show output is not valid JSON: %v\n%s", err, got)
	}
}

// TestAPISet verifies that "api set" updates a field and the change persists.
func TestAPISet(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://old.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o644)

	// Set a new base_url.
	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", "base_url", "https://new.example.com"}); err != nil {
		t.Fatalf("api set: %v", err)
	}

	// Reload and verify persistence.
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].BaseURL; got != "https://new.example.com" {
		t.Errorf("base_url after set: got %q, want https://new.example.com", got)
	}
}

func TestAPISetPreservesJSONCComments(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  // API registrations
  "apis": {
    // Main API
    "myapi": {
      "base_url": "https://old.example.com" // keep this note
    }
  }
}`)

	c, _, errOut := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", "base_url", "https://new.example.com"}); err != nil {
		t.Fatalf("api set: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "// API registrations") {
		t.Fatalf("expected top-level comment to be preserved:\n%s", got)
	}
	if !strings.Contains(got, "// Main API") {
		t.Fatalf("expected member comment to be preserved:\n%s", got)
	}
	if !strings.Contains(got, "// keep this note") {
		t.Fatalf("expected inline comment to be preserved:\n%s", got)
	}
	if strings.Contains(errOut.String(), "will not be preserved") {
		t.Fatalf("did not expect comment-loss warning, got %q", errOut.String())
	}
	if !strings.Contains(got, "https://new.example.com") {
		t.Fatalf("expected updated value in file:\n%s", got)
	}
}

func TestAPISetCreatesNestedJSONCPath(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    // Main API
    "myapi": {
      "base_url": "https://api.example.com"
    }
  }
}`)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", "profiles.default.auth.params.token", "secret"}); err != nil {
		t.Fatalf("api set nested: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "// Main API") {
		t.Fatalf("expected existing comment to be preserved:\n%s", got)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].Profiles["default"].Auth.Params["token"]; got != "secret" {
		t.Fatalf("token after set: got %q, want secret", got)
	}
}

func TestAPISetShorthandExpression(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://old.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `allow_cross_origin_spec: true`}); err != nil {
		t.Fatalf("api set shorthand: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !written.APIs["myapi"].AllowCrossOriginSpec {
		t.Fatalf("expected allow_cross_origin_spec to be true")
	}
}

func TestAPISetMultipleShorthandExpressions(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://old.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `allow_cross_origin_spec: true`, `pagination.items_path: "items"`}); err != nil {
		t.Fatalf("api set multi shorthand: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !written.APIs["myapi"].AllowCrossOriginSpec {
		t.Fatalf("expected allow_cross_origin_spec to be true")
	}
	if got := written.APIs["myapi"].Pagination.ItemsPath; got != "items" {
		t.Fatalf("expected pagination.items_path to be items, got %q", got)
	}
}

func TestAPISetShorthandAppendHeaders(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `profiles.default.headers[]: "Authorization: Bearer abc"`}); err != nil {
		t.Fatalf("api set append headers: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].Profiles["default"].Headers; len(got) != 1 || got[0] != "Authorization: Bearer abc" {
		t.Fatalf("unexpected headers after append: %#v", got)
	}
}

func TestAPISetShorthandDeleteKey(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com", OperationBase: "/v1"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `operation_base: undefined`}); err != nil {
		t.Fatalf("api set delete key: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].OperationBase; got != "" {
		t.Fatalf("expected operation_base to be deleted, got %q", got)
	}
}

func TestAPISetRejectsUnknownAuthType(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	err := c.Run([]string{"restish", "api", "set", "myapi", `profiles.default.auth.type: "oauth-typo"`})
	if err == nil {
		t.Fatal("expected auth.type validation error")
	}
	if !strings.Contains(err.Error(), "invalid auth.type") {
		t.Fatalf("expected invalid auth.type error, got: %v", err)
	}
}

func TestAPISetRejectsUnknownTLSSigner(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	err := c.Run([]string{"restish", "api", "set", "myapi", `profiles.default.tls_signer: "not-a-plugin"`})
	if err == nil {
		t.Fatal("expected tls_signer validation error")
	}
	if !strings.Contains(err.Error(), "not a registered tls-signer plugin") {
		t.Fatalf("expected tls_signer validation error, got: %v", err)
	}
}

func TestAPISetMixedShorthandPreservesComments(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  // API registrations
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com" // important note
    }
  }
}`)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{
		"restish", "api", "set", "myapi",
		`profiles.default.headers[]: "X-Test: 1"`,
		`allow_cross_origin_spec: true`,
		`operation_base: undefined`,
	}); err != nil {
		t.Fatalf("api set mixed shorthand: %v", err)
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "// API registrations") || !strings.Contains(got, "// important note") {
		t.Fatalf("expected comments preserved, got:\n%s", got)
	}
}

func TestAPIAddWithShorthand(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "add", "myapi", "https://api.example.com", `profiles.default.auth.type: "http-basic"`}); err != nil {
		t.Fatalf("api add shorthand: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].BaseURL; got != "https://api.example.com" {
		t.Fatalf("base_url after add: got %q", got)
	}
	if got := written.APIs["myapi"].Profiles["default"].Auth.Type; got != "http-basic" {
		t.Fatalf("auth.type after add: got %q", got)
	}
}

func TestAPIConfigurePreservesJSONCComments(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  // Existing APIs
  "apis": {
    "other": {
      "base_url": "https://other.example.com"
    }
  }
}`)

	c, out, _ := newTestCLI()
	c.ConfigPath = cfgFile
	c.SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case "https://api.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specWithXCLIConfig("https://api.example.com"))),
				Request:    r,
			}, nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})

	if err := c.Run([]string{"restish", "api", "configure", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api configure: %v", err)
	}
	if !strings.Contains(out.String(), "myapi") {
		t.Fatalf("expected configure output, got %q", out.String())
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "// Existing APIs") {
		t.Fatalf("expected existing comment to be preserved:\n%s", got)
	}
	if !strings.Contains(got, `"myapi"`) {
		t.Fatalf("expected new api entry:\n%s", got)
	}
}

func TestAPIConfigureDoesNotOverwriteInvalidConfig(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	invalid := "{\n  \"apis\": {\n"
	if err := os.WriteFile(cfgFile, []byte(invalid), 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	c, _, _ := newTestCLI()
	c.ConfigPath = cfgFile
	c.SpecCachePath = t.TempDir()

	err := c.Run([]string{"restish", "api", "configure", "myapi", "https://api.example.com"})
	if err == nil {
		t.Fatal("expected api configure to fail for invalid config")
	}
	if !strings.Contains(err.Error(), "invalid config") {
		t.Fatalf("expected invalid config error, got: %v", err)
	}

	data, readErr := os.ReadFile(cfgFile)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if string(data) != invalid {
		t.Fatalf("expected invalid config to remain unchanged, got:\n%s", data)
	}
}

func TestAPIDeletePreservesJSONCComments(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    // Keep this API
    "keep": {
      "base_url": "https://keep.example.com"
    },
    // Remove this API
    "remove": {
      "base_url": "https://remove.example.com"
    }
  }
}`)

	c, out, _ := newTestCLI()
	c.ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "delete", "remove"}); err != nil {
		t.Fatalf("api delete: %v", err)
	}
	if !strings.Contains(out.String(), "Deleted API") {
		t.Fatalf("expected delete output, got %q", out.String())
	}

	data, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "// Keep this API") {
		t.Fatalf("expected kept comment to remain:\n%s", got)
	}
	if strings.Contains(got, "remove.example.com") {
		t.Fatalf("expected API to be removed:\n%s", got)
	}
}

// TestAPISyncClearsCache (verifies api sync already tested in spec_test.go,
// but also that it reports success from the api subcommand path).
func TestAPISyncReportsSuccess(t *testing.T) {
	c := newSpecTestCLI(t, "syncapi", "https://api.example.com")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/openapi.json":
			return jsonResponse(200, minimalOpenAPI), nil
		default:
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		}
	})
	var out strings.Builder
	c.Stdout = &out
	if err := c.Run([]string{"restish", "api", "sync", "syncapi"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}
	if !strings.Contains(out.String(), "Synced") {
		t.Errorf("expected Synced in output, got: %q", out.String())
	}
}

// TestAPIContentTypes verifies that "api content-types" lists the built-in types.
func TestAPIContentTypes(t *testing.T) {
	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"

	if err := c.Run([]string{"restish", "api", "content-types"}); err != nil {
		t.Fatalf("api content-types: %v", err)
	}
	got := out.String()
	// JSON is always registered.
	if !strings.Contains(got, "json") {
		t.Errorf("expected json in content-types output, got: %q", got)
	}
}
