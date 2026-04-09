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
