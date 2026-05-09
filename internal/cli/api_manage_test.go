package cli_test

import (
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
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

// TestAPIConnect verifies that "api connect" fetches the spec, reads
// x-cli-config, and writes a config file with the pre-populated fields.
func TestAPIConnect(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	if !strings.Contains(out.String(), "myapi") {
		t.Errorf("expected confirmation message, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "Wrote config: "+cfgFile) {
		t.Errorf("expected written config path, got: %q", out.String())
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

func TestAPIConnectPrimesGeneratedHelp(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	cacheDir := t.TempDir()

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
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
				Body:       io.NopCloser(strings.NewReader(specWithOperations("https://api.example.com"))),
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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	helpCLI, helpOut, _ := newTestCLI(t)
	helpCLI.Hooks().ConfigPath = cfgFile
	helpCLI.Hooks().SpecCachePath = cacheDir
	useTransport(helpCLI, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("generated help should use the primed spec cache")
	})

	if err := helpCLI.Run([]string{"restish", "myapi", "--help"}); err != nil {
		t.Fatalf("generated help after api connect: %v", err)
	}
	help := helpOut.String()
	if !strings.Contains(help, "list-items") {
		t.Fatalf("expected generated operation help after api connect, got:\n%s", help)
	}
	if strings.Contains(help, "Generic requests using") {
		t.Fatalf("expected generated API help, got generic short-name help:\n%s", help)
	}
}

func TestAPIConnectExplicitSpecServedAsTextPlain(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://raw.example.com/openapi.yml":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
				Body:       io.NopCloser(strings.NewReader(specWithOperations("https://api.example.com"))),
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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com", "--spec", "https://raw.example.com/openapi.yml"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	if strings.Contains(out.String(), "no spec found") {
		t.Fatalf("expected text/plain explicit spec to be discovered, got:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "operations discovered") {
		t.Fatalf("expected discovered operations, got:\n%s", out.String())
	}
}

func TestAPIConnectPreservesEmbedderDefaultConfig(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.SetDefaultConfig(&config.Config{
		APIs: map[string]*config.APIConfig{
			"embedded": {
				BaseURL: "https://embedded.example.com",
			},
		},
	})

	if err := c.Run([]string{"restish", "api", "connect", "newapi", "https://api.example.com", "--no-discover"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	loaded := c.Config()
	if loaded == nil {
		t.Fatal("expected loaded config")
	}
	if loaded.APIs["embedded"] == nil {
		t.Fatalf("expected embedded default API to remain in c.cfg, got %#v", loaded.APIs)
	}
	if loaded.APIs["newapi"] == nil {
		t.Fatalf("expected connected API in c.cfg, got %#v", loaded.APIs)
	}
}

func TestAPIConnectFindsWellKnownOfficialOpenAPISpec(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	specBody := `{"components":{"schemas":{"Thing":{"type":"object"}}},"info":{"title":"Managed API","version":"1.0"},"paths":{"/things":{"get":{"operationId":"list-things","responses":{"200":{"description":"OK"}}}}},"openapi":"3.1.0"}`

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/vnd.oai.openapi+json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
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

	if err := c.Run([]string{"restish", "api", "connect", "example", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	if strings.Contains(out.String(), "no spec found") {
		t.Fatalf("expected spec to be found, got: %q", out.String())
	}
	if !strings.Contains(out.String(), "operations discovered") {
		t.Fatalf("expected discovered operations message, got: %q", out.String())
	}
}

func TestAPISyncDiscoveryUsesProfileCACert(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, specWithXCLIConfig("https://api.example.com"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
		t.Fatalf("write CA: %v", err)
	}

	cfgFile := filepath.Join(t.TempDir(), "restish.json")
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"secure": {
				BaseURL: srv.URL,
				Profiles: map[string]*config.ProfileConfig{
					"default": {CACertPath: caPath},
				},
			},
		},
	})
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	if err := c.Run([]string{"restish", "api", "sync", "secure"}); err != nil {
		t.Fatalf("api sync with profile CA: %v", err)
	}
}

func TestAPIConnectAllowCrossOriginSpec(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com", "--allow-cross-origin-spec"}); err != nil {
		t.Fatalf("api connect: %v", err)
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

func TestAPIConnectPreservesExistingProfilesByDefault(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()

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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("first api connect: %v", err)
	}

	currentHeader = "Accept: application/vnd.api+json"
	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("second api connect: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if got, want := written.APIs["myapi"].Profiles["default"].Headers[0], "Accept: application/json"; got != want {
		t.Fatalf("expected existing profile header %q to be preserved, got %q", want, got)
	}
}

func TestAPIConnectReplaceRefreshesProfiles(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()

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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("first api connect: %v", err)
	}

	currentHeader = "Accept: application/vnd.api+json"
	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com", "--replace"}); err != nil {
		t.Fatalf("second api connect: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if got := written.APIs["myapi"].Profiles["default"].Headers[0]; got != currentHeader {
		t.Fatalf("expected refreshed x-cli-config header %q, got %q", currentHeader, got)
	}
}

func TestAPIConnectLegacyXCLIConfigPrompt(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "components": {
    "securitySchemes": {
      "default": {
        "type": "oauth2",
        "flows": {
          "clientCredentials": {
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {}
          }
        }
      }
    }
  },
  "x-cli-config": {
    "security": "default",
    "headers": {"X-Org": "{org}"},
    "prompt": {
      "client_id": {"description": "Client identifier", "example": "abc123"},
      "org": {"description": "Organization", "exclude": true}
    },
    "params": {
      "audience": "https://example.com/{org}"
    }
  },
  "paths": {}
}`

	c, _, stderr := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("abc123\nacme\n")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	if !strings.Contains(stderr.String(), "Client identifier") || !strings.Contains(stderr.String(), "Organization") {
		t.Fatalf("expected connect-time prompts, got stderr %q", stderr.String())
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	prof := written.APIs["myapi"].Profiles["default"]
	if prof == nil || prof.Auth == nil {
		t.Fatalf("expected default auth profile, got %#v", prof)
	}
	if got := prof.Auth.Params["client_id"]; got != "abc123" {
		t.Fatalf("client_id = %q, want abc123", got)
	}
	if _, ok := prof.Auth.Params["org"]; ok {
		t.Fatalf("excluded prompt value was saved in auth params: %#v", prof.Auth.Params)
	}
	if got := prof.Auth.Params["audience"]; got != "https://example.com/acme" {
		t.Fatalf("audience = %q, want rendered org audience", got)
	}
	if got := prof.Headers; len(got) != 1 || got[0] != "X-Org: acme" {
		t.Fatalf("headers = %#v", got)
	}
	if prof.Credentials["default"] == nil || prof.Credentials["default"].Auth == nil {
		t.Fatalf("expected legacy x-cli-config security to also write credential binding, got %#v", prof.Credentials)
	}
}

func TestAPIConnectRetriesInvalidXCLIPromptInput(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "x-cli-config": {
    "profiles": {
      "default": {
        "headers": ["X-Env: {environment}"],
        "prompt": {
          "environment": {"description": "Environment", "enum": ["prod", "stage"]}
        }
      }
    }
  },
  "paths": {}
}`

	c, _, stderr := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("\nqa\nprod\n")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/openapi.json" {
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	errText := stderr.String()
	if !strings.Contains(errText, "environment is required; please enter a non-empty value.") {
		t.Fatalf("expected required-value retry guidance, got %q", errText)
	}
	if !strings.Contains(errText, "environment must be one of: prod, stage.") {
		t.Fatalf("expected enum retry guidance, got %q", errText)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	if got := written.APIs["myapi"].Profiles["default"].Headers; len(got) != 1 || got[0] != "X-Env: prod" {
		t.Fatalf("headers = %#v", got)
	}
}

func TestAPIConnectXCLIConfigCredentialPrompt(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "x-cli-config": {
    "profiles": {
      "default": {
        "credentials": {
          "PartnerKey": {
            "auth": {
              "type": "api-key",
              "params": {"in": "header", "name": "X-Partner-Key", "value": "{partner_key}"}
            },
            "prompt": {
              "partner_key": {"description": "Partner API key"}
            },
            "satisfies": ["reports:read"]
          }
        }
      }
    }
  },
  "paths": {}
}`

	c, _, stderr := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("secret-key\n")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/openapi.json" {
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	if !strings.Contains(stderr.String(), "Partner API key") {
		t.Fatalf("expected credential prompt, got stderr %q", stderr.String())
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	credential := written.APIs["myapi"].Profiles["default"].Credentials["PartnerKey"]
	if credential == nil || credential.Auth == nil {
		t.Fatalf("credential = %#v", credential)
	}
	if credential.Auth.Params["value"] != "secret-key" {
		t.Fatalf("auth params = %#v", credential.Auth.Params)
	}
	if got := credential.Satisfies; !reflect.DeepEqual(got, []string{"reports:read"}) {
		t.Fatalf("satisfies = %#v", got)
	}
}

func TestAPIConnectV2ProfilePromptShape(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "x-cli-config": {
    "profiles": {
      "default": {
        "headers": ["X-Org: {org}", "Authorization: Bearer {auth_token}"],
        "auth": {
          "type": "bearer",
          "params": {
            "token": "{auth_token}",
            "audience": "https://example.com/{org}"
          }
        },
        "params": {
          "region": "{org}-west"
        },
        "prompt": {
          "auth_token": {"description": "API token"},
          "org": {"description": "Organization"}
        }
      }
    }
  },
  "paths": {}
}`

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("tok-{org}\nacme\n")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/openapi.json" {
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	prof := written.APIs["myapi"].Profiles["default"]
	if prof == nil || prof.Auth == nil {
		t.Fatalf("expected default auth profile, got %#v", prof)
	}
	for _, want := range []string{"X-Org: acme", "Authorization: Bearer tok-{org}"} {
		if !containsString(prof.Headers, want) {
			t.Fatalf("headers missing %q: %#v", want, prof.Headers)
		}
	}
	for key, want := range map[string]string{
		"auth_token": "tok-{org}",
		"org":        "acme",
		"region":     "acme-west",
		"token":      "tok-{org}",
		"audience":   "https://example.com/acme",
	} {
		if got := prof.Auth.Params[key]; got != want {
			t.Fatalf("auth param %s = %q, want %q (all params %#v)", key, got, want, prof.Auth.Params)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// TestAPIInspect verifies that "api inspect" prints the API config as JSON.
func TestAPIInspect(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile

	if err := c.Run([]string{"restish", "api", "inspect", "myapi"}); err != nil {
		t.Fatalf("api inspect: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "api.example.com") {
		t.Errorf("expected base_url in output, got: %q", got)
	}
	// Validate that output is valid JSON.
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(got)), &parsed); err != nil {
		t.Errorf("api inspect output is not valid JSON: %v\n%s", err, got)
	}
}

func TestAPIInspectHighlightsJSONWhenColorEnabled(t *testing.T) {
	t.Setenv("NOCOLOR", "")
	t.Setenv("NO_COLOR", "")
	t.Setenv("COLOR", "1")

	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile

	if err := c.Run([]string{"restish", "api", "inspect", "myapi"}); err != nil {
		t.Fatalf("api inspect: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI highlighting, got %q", got)
	}
	stripped := stripANSI(got)
	var parsed map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stripped)), &parsed); err != nil {
		t.Fatalf("api inspect output is not valid JSON after stripping ANSI: %v\n%s", err, stripped)
	}
	if !strings.Contains(stripped, "api.example.com") {
		t.Fatalf("expected base_url in output, got: %q", stripped)
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
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	// Set a new base_url.
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `base_url: https://new.example.com`}); err != nil {
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

	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `base_url: https://new.example.com`}); err != nil {
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `profiles.default.auth.params.token: secret`}); err != nil {
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

func TestAPISetCreatesCredentialPath(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "auth_profiles": {
    "shared": {
      "type": "oauth-client-credentials"
    }
  },
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com"
    }
  }
}`)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{
		"restish", "api", "set", "myapi",
		"profiles.default.credentials.UserOAuth.auth_ref: shared",
		`profiles.default.credentials.UserOAuth.satisfies: ["items:read","items:write"]`,
	}); err != nil {
		t.Fatalf("api set credential: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	credential := written.APIs["myapi"].Profiles["default"].Credentials["UserOAuth"]
	if credential == nil {
		t.Fatal("expected UserOAuth credential")
	}
	if credential.AuthRef != "shared" {
		t.Fatalf("AuthRef = %q, want shared", credential.AuthRef)
	}
	if got, want := credential.Satisfies, []string{"items:read", "items:write"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Satisfies = %#v, want %#v", got, want)
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
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

func TestAPISetFullAuthObject(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{
		"restish", "api", "set", "myapi",
		`profiles.demo.auth: {type: http-basic, params: {username: demo, password: env:DEMO_PASSWORD}}`,
	}); err != nil {
		t.Fatalf("api set full auth object: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	auth := written.APIs["myapi"].Profiles["demo"].Auth
	if auth == nil || auth.Type != "http-basic" || auth.Params["username"] != "demo" || auth.Params["password"] != "env:DEMO_PASSWORD" {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestAPISetRootedObjectPatch(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{
		"restish", "api", "set", "myapi",
		`{profiles: {demo: {headers: ["X-Debug: true"]}}}`,
	}); err != nil {
		t.Fatalf("api set rooted object patch: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	got := written.APIs["myapi"].Profiles["demo"].Headers
	if !reflect.DeepEqual(got, []string{"X-Debug: true"}) {
		t.Fatalf("headers = %#v", got)
	}
}

func TestAPISetRejectsNonPatchForm(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	err := c.Run([]string{"restish", "api", "set", "myapi", "base_url", "https://new.example.com"})
	if err == nil {
		t.Fatal("expected non-patch form to be rejected")
	}
	if !strings.Contains(err.Error(), "expected shorthand patch expression") {
		t.Fatalf("unexpected error: %v", err)
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
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

func TestAPISetValidatesOperationBasePath(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "set", "myapi", `operation_base: "/v1"`}); err != nil {
		t.Fatalf("expected absolute path operation_base to be accepted: %v", err)
	}
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].OperationBase; got != "/v1" {
		t.Fatalf("OperationBase = %q, want /v1", got)
	}

	err = c.Run([]string{"restish", "api", "set", "myapi", `operation_base: "v1"`})
	if err == nil {
		t.Fatal("expected relative operation_base to be rejected")
	}
	if !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("unexpected error: %v", err)
	}

	err = c.Run([]string{"restish", "api", "set", "myapi", `operation_base: "https://api.example.com/v1"`})
	if err == nil {
		t.Fatal("expected URL operation_base to be rejected")
	}
	if !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPISetServerVariables(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "staging": {}
      }
    }
  }
}`)
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile

	if err := c.Run([]string{"restish", "api", "set", "myapi", `server_variables.env: staging`, `profiles.staging.server_variables.version: v2`}); err != nil {
		t.Fatalf("api set server variables: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load written config: %v", err)
	}
	api := written.APIs["myapi"]
	if got := api.ServerVariables["env"]; got != "staging" {
		t.Fatalf("server_variables.env = %q, want staging", got)
	}
	if got := api.Profiles["staging"].ServerVariables["version"]; got != "v2" {
		t.Fatalf("profiles.staging.server_variables.version = %q, want v2", got)
	}

	if err := c.Run([]string{"restish", "api", "set", "myapi", `server_variables.env: undefined`, `profiles.staging.server_variables.version: undefined`}); err != nil {
		t.Fatalf("api remove server variables: %v", err)
	}
	written, err = config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload written config: %v", err)
	}
	if _, ok := written.APIs["myapi"].ServerVariables["env"]; ok {
		t.Fatal("expected server_variables.env to be deleted")
	}
	if prof := written.APIs["myapi"].Profiles["staging"]; prof != nil && prof.ServerVariables != nil {
		if _, ok := prof.ServerVariables["version"]; ok {
			t.Fatal("expected profile server variable to be deleted")
		}
	}
}

func TestAPISetInvalidatesSpecCacheForBaseFields(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)
	cacheDir := t.TempDir()
	cacheFile := cacheDir + "/myapi.cbor"
	if err := os.WriteFile(cacheFile, []byte("cached"), 0o600); err != nil {
		t.Fatal(err)
	}

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "api", "set", "myapi", `base_url: "https://new.example.com"`}); err != nil {
		t.Fatalf("api set: %v", err)
	}
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Fatalf("expected spec cache to be invalidated, stat err=%v", err)
	}
}

func TestAPISetDoesNotInvalidateSpecCacheForUnrelatedFields(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	_ = os.WriteFile(cfgFile, cfgData, 0o600)
	cacheDir := t.TempDir()
	cacheFile := cacheDir + "/myapi.cbor"
	if err := os.WriteFile(cacheFile, []byte("cached"), 0o600); err != nil {
		t.Fatal(err)
	}

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = cacheDir
	if err := c.Run([]string{"restish", "api", "set", "myapi", `profiles.default.headers[]: "X-Test: 1"`}); err != nil {
		t.Fatalf("api set: %v", err)
	}
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("expected spec cache to remain, stat err=%v", err)
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
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

func TestAPIConnectWithShorthand(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com", "--no-discover", `profiles.default.auth.type: "http-basic"`}); err != nil {
		t.Fatalf("api connect shorthand: %v", err)
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

func TestAPIConnectCreatesMissingConfigFile(t *testing.T) {
	cfgFile := filepath.Join(t.TempDir(), "restish.json")

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com", "--no-discover"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].BaseURL; got != "https://api.example.com" {
		t.Fatalf("base_url after add: got %q", got)
	}
}

func TestAPIConnectNormalizesSchemelessURL(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "connect", "remote", "api.example.com", "--no-discover"}); err != nil {
		t.Fatalf("api connect remote: %v", err)
	}
	if err := c.Run([]string{"restish", "api", "connect", "local", "localhost:8080", "--no-discover"}); err != nil {
		t.Fatalf("api connect local: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["remote"].BaseURL; got != "https://api.example.com" {
		t.Fatalf("remote base_url = %q, want https://api.example.com", got)
	}
	if got := written.APIs["local"].BaseURL; got != "http://localhost:8080" {
		t.Fatalf("local base_url = %q, want http://localhost:8080", got)
	}
}

func TestAPIConnectNoDiscoverPerformsNoNetworkDiscovery(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request during --no-discover: %s", r.URL)
		return nil, nil
	})

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com", "--no-discover"}); err != nil {
		t.Fatalf("api connect --no-discover: %v", err)
	}
	if !strings.Contains(out.String(), "discovery skipped") {
		t.Fatalf("expected discovery skipped summary, got %q", out.String())
	}
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].BaseURL; got != "https://api.example.com" {
		t.Fatalf("base_url = %q", got)
	}
}

func TestAPIConnectSpecLocalFile(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)
	specFile := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specFile, []byte(`{
		"openapi":"3.1.0",
		"info":{"title":"Local","version":"1.0"},
		"paths":{"/items":{"get":{"operationId":"list-items","responses":{"200":{"description":"OK"}}}}}
	}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected network request for local --spec: %s", r.URL)
		return nil, nil
	})

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com", "--spec", specFile}); err != nil {
		t.Fatalf("api connect --spec file: %v", err)
	}
	if !strings.Contains(out.String(), "1 operations discovered") {
		t.Fatalf("expected operation count in summary, got %q", out.String())
	}
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := written.APIs["myapi"].SpecFiles; !reflect.DeepEqual(got, []string{specFile}) {
		t.Fatalf("spec_files = %#v, want %q", got, specFile)
	}
}

func TestAPIConnectSetupExpressions(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "x-cli-config": {
    "profiles": {
      "default": {
        "headers": ["X-Client: {client_id}"],
        "auth": {"type": "bearer", "params": {"token": "{token}"}},
        "prompt": {
          "client_id": {"description": "Client ID"},
          "token": {"description": "Token"}
        }
      }
    }
  },
  "paths": {}
}`

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
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

	if err := c.Run([]string{
		"restish", "api", "connect", "myapi", "api.example.com",
		`prompt.client_id: abc123`,
		`prompt.token: secret-token`,
		`profiles.default.headers[]: "X-Env: prod"`,
	}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	api := written.APIs["myapi"]
	if got := api.BaseURL; got != "https://api.example.com" {
		t.Fatalf("base_url = %q, want https://api.example.com", got)
	}
	prof := api.Profiles["default"]
	for _, want := range []string{"X-Client: abc123", "X-Env: prod"} {
		if !containsString(prof.Headers, want) {
			t.Fatalf("headers missing %q: %#v", want, prof.Headers)
		}
	}
	if got := prof.Auth.Params["token"]; got != "secret-token" {
		t.Fatalf("token = %q, want secret-token", got)
	}
}

func TestAPIConnectFallbackAPIKeySetup(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "components": {
    "securitySchemes": {
      "key": {"type": "apiKey", "in": "header", "name": "X-API-Key"}
    }
  },
  "paths": {}
}`

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com/openapi.json":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
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

	if err := c.Run([]string{
		"restish", "api", "connect", "myapi", "api.example.com",
		`prompt.value: secret-key`,
	}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	prof := written.APIs["myapi"].Profiles["default"]
	if prof.Auth == nil || prof.Auth.Type != "api-key" || prof.Auth.Params["value"] != "secret-key" {
		t.Fatalf("api key fallback auth = %#v", prof.Auth)
	}
	if prof.Credentials["key"] == nil || prof.Credentials["key"].Auth == nil || prof.Credentials["key"].Auth.Type != "api-key" {
		t.Fatalf("api key fallback credential = %#v", prof.Credentials)
	}
}

func TestAPIConnectFallbackHTTPBasicPromptsCredentials(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "components": {
    "securitySchemes": {
      "basicAuth": {"type": "http", "scheme": "basic"}
    }
  },
  "security": [{"basicAuth": []}],
  "paths": {}
}`

	c, _, stderr := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("alice\nsecret\n")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/openapi.json" {
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	if !strings.Contains(stderr.String(), "Username:") || !strings.Contains(stderr.String(), "Password:") {
		t.Fatalf("expected basic auth prompts, got stderr %q", stderr.String())
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	prof := written.APIs["myapi"].Profiles["default"]
	if prof.Auth == nil || prof.Auth.Type != "http-basic" || prof.Auth.Params["username"] != "alice" || prof.Auth.Params["password"] != "secret" {
		t.Fatalf("basic fallback auth = %#v", prof.Auth)
	}
	if prof.Credentials["basicAuth"] == nil || prof.Credentials["basicAuth"].Auth.Params["username"] != "alice" {
		t.Fatalf("basic fallback credential = %#v", prof.Credentials)
	}
}

func TestAPIConnectFallbackMultiCredentialSetup(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Managed API", "version": "1.0"},
  "components": {
    "securitySchemes": {
      "UserOAuth": {
        "type": "oauth2",
        "flows": {
          "clientCredentials": {
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {"items:read": "Read items"}
          }
        }
      },
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"}
    }
  },
  "security": [{"UserOAuth": ["items:read"]}, {"PartnerKey": []}],
  "paths": {}
}`

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("n\n")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/openapi.json" {
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{
		"restish", "api", "connect", "myapi", "api.example.com",
		`prompt.credentials.PartnerKey.value: partner-secret`,
	}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	if !strings.Contains(out.String(), "Auth coverage") {
		t.Fatalf("expected coverage summary, got %q", out.String())
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	creds := written.APIs["myapi"].Profiles["default"].Credentials
	if creds["UserOAuth"] != nil {
		t.Fatalf("expected UserOAuth to be skipped without an answer, got %#v", creds["UserOAuth"])
	}
	if creds["PartnerKey"] == nil || creds["PartnerKey"].Auth == nil || creds["PartnerKey"].Auth.Params["value"] != "partner-secret" {
		t.Fatalf("PartnerKey credential = %#v", creds["PartnerKey"])
	}
}

func TestAPIConnectFallbackAuthDiscoveryFlow(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{}`)
	specBody := `{
  "openapi": "3.1.0",
  "info": {"title": "Example API", "version": "1.0"},
  "components": {
    "securitySchemes": {
      "UserOAuth": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {
            "authorizationUrl": "https://auth.example.com/authorize",
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {
              "items:read": "Read items",
              "items:write": "Write items"
            }
          }
        }
      },
      "AdminOAuth": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {
            "authorizationUrl": "https://auth.example.com/admin/authorize",
            "tokenUrl": "https://auth.example.com/admin/token",
            "scopes": {"admin:read": "Read admin"}
          }
        }
      },
      "PartnerKey": {"type": "apiKey", "in": "header", "name": "X-Partner-Key"}
    }
  },
  "security": [{"UserOAuth": ["items:read", "items:write"]}],
  "paths": {
    "/items": {"get": {"operationId": "list-items", "responses": {"200": {"description": "OK"}}}},
    "/admin": {"get": {"operationId": "get-admin", "security": [{"AdminOAuth": ["admin:read"]}], "responses": {"200": {"description": "OK"}}}},
    "/partner": {"get": {"operationId": "get-partner", "security": [{"PartnerKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/either": {"get": {"operationId": "get-either", "security": [{"UserOAuth": ["items:read"]}, {"PartnerKey": []}], "responses": {"200": {"description": "OK"}}}},
    "/public": {"get": {"operationId": "get-public", "security": [], "responses": {"200": {"description": "OK"}}}}
  }
}`

	c, out, stderr := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("y\nuser-client\n\ny\nadmin-client\n\nn\n")
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		if r.URL.String() == "https://api.example.com/openapi.json" {
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(specBody)),
				Request:    r,
			}, nil
		}
		return &http.Response{
			StatusCode: 404,
			Proto:      "HTTP/1.1",
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("not found")),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "api", "connect", "example", "api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	outText := out.String()
	for _, want := range []string{
		"Discovered Example API",
		"This API declares 3 auth scheme(s)",
		"UserOAuth",
		"global default",
		"configured: AdminOAuth, UserOAuth",
		"skipped:    PartnerKey",
		"callable:   3/4 secured operations",
	} {
		if !strings.Contains(outText, want) {
			t.Fatalf("expected stdout to contain %q, got:\n%s", want, outText)
		}
	}
	errText := stderr.String()
	for _, want := range []string{
		"Configure UserOAuth? [Y/n]",
		"Client ID:",
		"Scopes [items:read items:write]:",
		"Configure AdminOAuth? [y/N]",
		"Configure PartnerKey? [y/N]",
	} {
		if !strings.Contains(errText, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, errText)
		}
	}

	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	prof := written.APIs["example"].Profiles["default"]
	if prof.Auth == nil || prof.Auth.Type != "oauth-authorization-code" || prof.Auth.Params["client_id"] != "user-client" {
		t.Fatalf("profile auth = %#v", prof.Auth)
	}
	user := prof.Credentials["UserOAuth"]
	if user == nil || user.Auth == nil || user.Auth.Params["client_id"] != "user-client" {
		t.Fatalf("UserOAuth = %#v", user)
	}
	if got := user.Satisfies; !reflect.DeepEqual(got, []string{"items:read", "items:write"}) {
		t.Fatalf("UserOAuth satisfies = %#v", got)
	}
	admin := prof.Credentials["AdminOAuth"]
	if admin == nil || admin.Auth == nil || admin.Auth.Params["client_id"] != "admin-client" {
		t.Fatalf("AdminOAuth = %#v", admin)
	}
	if prof.Credentials["PartnerKey"] != nil {
		t.Fatalf("expected PartnerKey to be skipped, got %#v", prof.Credentials["PartnerKey"])
	}
}

func TestAPIConnectPreservesJSONCComments(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  // Existing APIs
  "apis": {
    "other": {
      "base_url": "https://other.example.com"
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()
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

	if err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}
	if !strings.Contains(out.String(), "myapi") {
		t.Fatalf("expected connect output, got %q", out.String())
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

func TestAPIConnectDoesNotOverwriteInvalidConfig(t *testing.T) {
	cfgFile := t.TempDir() + "/restish.json"
	invalid := "{\n  \"apis\": {\n"
	if err := os.WriteFile(cfgFile, []byte(invalid), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	c.Hooks().SpecCachePath = t.TempDir()

	err := c.Run([]string{"restish", "api", "connect", "myapi", "https://api.example.com"})
	if err == nil {
		t.Fatal("expected api connect to fail for invalid config")
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

func TestAPIConnectAdversarialSpecShapesFailGracefully(t *testing.T) {
	deep := `{"type":"object"}`
	for i := 0; i < 64; i++ {
		deep = fmt.Sprintf(`{"type":"object","properties":{"n":%s}}`, deep)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
  "openapi": "3.1.0",
  "info": {"title": "Adversarial", "version": "1.0"},
  "servers": [{"url": %q}],
  "components": {
    "securitySchemes": {
      "OddOAuth": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {"authorizationUrl": "https://auth.example.com/auth", "scopes": {"read": "Read"}},
          "clientCredentials": {"tokenUrl": "https://auth.example.com/token", "scopes": {"write": "Write"}}
        }
      }
    },
    "schemas": {"Deep": %s}
  },
  "paths": {
    "/items": {
      "post": {
        "operationId": "createItem",
        "requestBody": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Deep"}}}},
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, "https://api.example.com", deep)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, _, _ := newTestCLI(t)
	c.Hooks().SpecCachePath = t.TempDir()
	c.Hooks().PassReader = strings.NewReader("client-id\n")
	err := c.Run([]string{"restish", "api", "connect", "adversarial", "https://api.example.com", "--spec", srv.URL + "/openapi.json"})
	if err != nil {
		t.Fatalf("api connect should handle adversarial-but-parseable spec gracefully: %v", err)
	}
}

func TestAPIConnectRejectsRemovedCommandNames(t *testing.T) {
	for _, args := range [][]string{
		{"restish", "api", "add", "myapi", "https://api.example.com"},
		{"restish", "api", "configure", "myapi", "https://api.example.com"},
		{"restish", "api", "delete", "myapi"},
	} {
		c, _, _ := newTestCLI(t)
		if err := c.Run(args); err == nil {
			t.Fatalf("%v: expected unknown command error", args)
		}
	}
}

func TestAPIRemovePreservesJSONCComments(t *testing.T) {
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

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "remove", "remove"}); err != nil {
		t.Fatalf("api remove: %v", err)
	}
	if !strings.Contains(out.String(), "Removed API") {
		t.Fatalf("expected remove output, got %q", out.String())
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

func TestAPISyncNetworkFailureLeavesRegistrationAndCache(t *testing.T) {
	c := newSpecTestCLI(t, "syncapi", "https://api.example.com")
	cacheFile := filepath.Join(c.Hooks().SpecCachePath, "syncapi.cbor")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("existing-cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})

	err := c.Run([]string{"restish", "api", "sync", "syncapi"})
	if err == nil {
		t.Fatal("expected api sync failure")
	}
	if !strings.Contains(err.Error(), "left unchanged") {
		t.Fatalf("expected unchanged hint, got %v", err)
	}
	if _, statErr := os.Stat(cacheFile); statErr != nil {
		t.Fatalf("expected existing cache to remain: %v", statErr)
	}
	cfg, loadErr := config.Load(c.Hooks().ConfigPath)
	if loadErr != nil {
		t.Fatalf("load config: %v", loadErr)
	}
	if cfg.APIs["syncapi"] == nil {
		t.Fatal("expected API registration to remain")
	}
}

// TestAPIContentTypes verifies that "content-types" lists the built-in types.
func TestAPIContentTypes(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"

	if err := c.Run([]string{"restish", "content-types"}); err != nil {
		t.Fatalf("content-types: %v", err)
	}
	got := out.String()
	// JSON is always registered.
	if !strings.Contains(got, "json") {
		t.Errorf("expected json in content-types output, got: %q", got)
	}
}
