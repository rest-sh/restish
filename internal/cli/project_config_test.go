package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	restishcli "github.com/rest-sh/restish/v2/internal/cli"
)

func newProjectConfigTestCLI(t *testing.T) (*restishcli.CLI, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	c := restishcli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().PassReader = strings.NewReader("")
	c.Hooks().RetryBaseDelay = time.Millisecond
	return c, &stdout, &stderr
}

func setupProjectConfigTest(t *testing.T) (configDir, cacheDir, projectDir string) {
	t.Helper()
	root := t.TempDir()
	configDir = filepath.Join(root, "config")
	cacheDir = filepath.Join(root, "cache")
	projectDir = filepath.Join(root, "repo")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	t.Setenv("RSH_CONFIG_DIR", configDir)
	t.Setenv("RSH_CACHE_DIR", cacheDir)
	return configDir, cacheDir, projectDir
}

func writeProjectConfigTestFile(t *testing.T, path, body string) {
	t.Helper()
	writeProjectConfigTestFileMode(t, path, body, 0o600)
}

func writeProjectConfigTestFileMode(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestProjectConfigRequiresTrustBeforeLayering(t *testing.T) {
	configDir, _, projectDir := setupProjectConfigTest(t)
	writeProjectConfigTestFile(t, filepath.Join(configDir, "restish.json"), `{
  "apis": {
    "global": {"base_url": "https://global.example.com"}
  }
}`)
	writeProjectConfigTestFile(t, filepath.Join(projectDir, ".restish.json"), `{
  "apis": {
    "project": {"base_url": "https://project.example.com"}
  }
}`)
	t.Chdir(projectDir)

	c, out, stderr := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "show"}); err != nil {
		t.Fatalf("config show: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "global") || strings.Contains(got, "project.example.com") {
		t.Fatalf("config show output = %q, want global only", got)
	}
	if got := stderr.String(); !strings.Contains(got, "is not trusted") || !strings.Contains(got, "config trust") {
		t.Fatalf("stderr = %q, want untrusted project config warning", got)
	}
}

func TestConfigTrustLayersProjectConfigAndRetrustsOnChange(t *testing.T) {
	configDir, _, projectDir := setupProjectConfigTest(t)
	writeProjectConfigTestFile(t, filepath.Join(configDir, "restish.json"), `{
  "apis": {
    "svc": {"base_url": "https://global.example.com"},
    "global": {"base_url": "https://only-global.example.com"}
  },
  "theme": {"string": "#00ff00"}
}`)
	projectPath := filepath.Join(projectDir, ".restish.json")
	writeProjectConfigTestFile(t, projectPath, `{
  "apis": {
    "svc": {"base_url": "https://project.example.com"}
  },
  "theme": {"keyword": "#ff00ff"}
}`)
	t.Chdir(filepath.Join(projectDir))

	c, out, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Trusted project config") || !strings.Contains(got, "svc") {
		t.Fatalf("config trust output = %q", got)
	}

	c, out, _ = newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "show"}); err != nil {
		t.Fatalf("trusted config show: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Project config:") || !strings.Contains(got, projectPath) {
		t.Fatalf("config show output = %q, want project config line", got)
	}
	if !strings.Contains(got, "https://project.example.com") || strings.Contains(got, "https://global.example.com") {
		t.Fatalf("config show output = %q, want project API to shadow global API", got)
	}
	if !strings.Contains(got, "global") {
		t.Fatalf("config show output = %q, want unrelated global API preserved", got)
	}

	writeProjectConfigTestFile(t, projectPath, `{
  "apis": {
    "svc": {"base_url": "https://changed.example.com"}
  }
}`)
	c, out, stderr := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "show"}); err != nil {
		t.Fatalf("changed config show: %v", err)
	}
	if strings.Contains(out.String(), "changed.example.com") {
		t.Fatalf("changed untrusted project config was loaded: %q", out.String())
	}
	if got := stderr.String(); !strings.Contains(got, "is not trusted") {
		t.Fatalf("stderr = %q, want retrust warning", got)
	}
}

func TestProjectConfigShowOmitsEmptyAPIList(t *testing.T) {
	_, _, projectDir := setupProjectConfigTest(t)
	projectPath := filepath.Join(projectDir, ".restish.json")
	writeProjectConfigTestFile(t, projectPath, `{
  "theme": {"keyword": "#ff00ff"}
}`)
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}

	c, out, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "show"}); err != nil {
		t.Fatalf("config show: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Project config: ") || !strings.Contains(got, ".restish.json") {
		t.Fatalf("config show output = %q, want project config path", got)
	}
	if strings.Contains(got, "()") {
		t.Fatalf("config show output = %q, want no empty project API list", got)
	}
}

func TestProjectConfigRejectsEmptyUnsupportedTopLevelKeys(t *testing.T) {
	_, _, projectDir := setupProjectConfigTest(t)
	writeProjectConfigTestFile(t, filepath.Join(projectDir, ".restish.json"), `{
  "apis": {},
  "auth_profiles": {}
}`)
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	err := c.Run([]string{"restish", "config", "trust"})
	if err == nil || !strings.Contains(err.Error(), "only apis and theme are supported") {
		t.Fatalf("config trust err = %v, want unsupported top-level key error", err)
	}
}

func TestProjectConfigAllowsCommittedFileWithEnvSecretReferences(t *testing.T) {
	_, _, projectDir := setupProjectConfigTest(t)
	writeProjectConfigTestFileMode(t, filepath.Join(projectDir, ".restish.json"), `{
  "apis": {
    "svc": {
      "base_url": "https://project.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "oauth-client-credentials",
            "params": {
              "client_id": "public-client",
              "client_secret": "env:PROJECT_CLIENT_SECRET",
              "token_url": "https://auth.example.com/token",
              "audience": "https://api.example.com"
            }
          }
        }
      }
    }
  }
}`, 0o644)
	t.Chdir(projectDir)

	c, out, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Trusted project config") || !strings.Contains(got, "svc") {
		t.Fatalf("config trust output = %q", got)
	}
}

func TestProjectConfigRejectsInlineSecrets(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "auth secret param",
			body: `{
  "apis": {
    "svc": {
      "base_url": "https://project.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "oauth-client-credentials",
            "params": {
              "client_id": "public-client",
              "client_secret": "inline-secret",
              "token_url": "https://auth.example.com/token",
              "audience": "https://api.example.com"
            }
          }
        }
      }
    }
  }
}`,
			want: "params.client_secret",
		},
		{
			name: "credential-bearing header",
			body: `{
  "apis": {
    "svc": {
      "base_url": "https://project.example.com",
      "profiles": {
        "default": {
          "headers": ["Authorization: Bearer inline-secret"]
        }
      }
    }
  }
}`,
			want: "credential-bearing header",
		},
		{
			name: "credential-bearing query param",
			body: `{
  "apis": {
    "svc": {
      "base_url": "https://project.example.com",
      "profiles": {
        "default": {
          "query": ["api_key=inline-secret"]
        }
      }
    }
  }
}`,
			want: "credential-bearing query parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, projectDir := setupProjectConfigTest(t)
			writeProjectConfigTestFileMode(t, filepath.Join(projectDir, ".restish.json"), tt.body, 0o644)
			t.Chdir(projectDir)

			c, _, _ := newProjectConfigTestCLI(t)
			err := c.Run([]string{"restish", "config", "trust"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("config trust err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestProjectConfigRejectsUnknownAuthType(t *testing.T) {
	_, _, projectDir := setupProjectConfigTest(t)
	writeProjectConfigTestFile(t, filepath.Join(projectDir, ".restish.json"), `{
  "apis": {
    "svc": {
      "base_url": "https://project.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "mystery-auth",
            "params": {
              "token": "inline-secret"
            }
          }
        }
      }
    }
  }
}`)
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	err := c.Run([]string{"restish", "config", "trust"})
	if err == nil || !strings.Contains(err.Error(), "unknown auth type") || !strings.Contains(err.Error(), "apis.svc.profiles.default.auth") {
		t.Fatalf("config trust err = %v, want unknown auth type path", err)
	}
}

func TestProjectConfigAPIsAreReadOnlyForMutatingCommands(t *testing.T) {
	_, _, projectDir := setupProjectConfigTest(t)
	writeProjectConfigTestFile(t, filepath.Join(projectDir, ".restish.json"), `{
  "apis": {
    "project": {"base_url": "https://project.example.com"}
  }
}`)
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}

	c, _, _ = newProjectConfigTestCLI(t)
	err := c.Run([]string{"restish", "api", "set", "project", "base_url: https://other.example.com"})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("api set err = %v, want read-only project API error", err)
	}
}

func TestProjectConfigDoesNotPersistThroughGlobalAPIRemove(t *testing.T) {
	configDir, _, projectDir := setupProjectConfigTest(t)
	globalPath := filepath.Join(configDir, "restish.json")
	writeProjectConfigTestFile(t, globalPath, `{
  "apis": {
    "global": {"base_url": "https://global.example.com"}
  }
}`)
	writeProjectConfigTestFile(t, filepath.Join(projectDir, ".restish.json"), `{
  "apis": {
    "project": {"base_url": "https://project.example.com"}
  },
  "theme": {"keyword": "#ff00ff"}
}`)
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}
	c, _, _ = newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "api", "remove", "global"}); err != nil {
		t.Fatalf("api remove global: %v", err)
	}

	data, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("read global config: %v", err)
	}
	if strings.Contains(string(data), "project.example.com") || strings.Contains(string(data), "ff00ff") {
		t.Fatalf("global config persisted project config: %s", data)
	}
	if strings.Contains(string(data), "global.example.com") {
		t.Fatalf("global API was not removed: %s", data)
	}
}

func TestProjectConfigAPISyncWarnsWhenOperationOriginsNeedProjectEdit(t *testing.T) {
	_, _, projectDir := setupProjectConfigTest(t)
	var specURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
  "openapi": "3.1.0",
  "info": {"title": "Project API", "version": "1.0"},
  "servers": [{"url": %q}],
  "paths": {
    "/upload": {
      "post": {
        "operationId": "uploadFile",
        "servers": [{"url": "https://uploads.example.com"}],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`, strings.TrimSuffix(specURL, "/openapi.json"))
	}))
	defer server.Close()
	specURL = server.URL + "/openapi.json"

	projectPath := filepath.Join(projectDir, ".restish.json")
	writeProjectConfigTestFile(t, projectPath, fmt.Sprintf(`{
  "apis": {
    "svc": {
      "base_url": %q,
      "spec_url": %q
    }
  }
}`, server.URL, specURL))
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}

	c, out, stderr := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "api", "sync", "svc", "--yes"}); err != nil {
		t.Fatalf("api sync: %v", err)
	}
	if strings.Contains(out.String(), "Wrote config:") {
		t.Fatalf("api sync wrote read-only project config:\n%s", out.String())
	}
	gotErr := stderr.String()
	for _, want := range []string{"read-only project API", "allowed_operation_origins[]", projectPath, "--rsh-config"} {
		if !strings.Contains(gotErr, want) {
			t.Fatalf("stderr = %q, want %q", gotErr, want)
		}
	}
	if strings.Contains(gotErr, "restish api set") {
		t.Fatalf("stderr suggested mutating read-only project API:\n%s", gotErr)
	}
}

func TestProjectConfigCanReferenceGlobalAuthProfiles(t *testing.T) {
	configDir, _, projectDir := setupProjectConfigTest(t)
	writeProjectConfigTestFile(t, filepath.Join(configDir, "restish.json"), `{
  "auth_profiles": {
    "shared": {"type": "bearer", "params": {"token": "global-token"}}
  }
}`)
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()
	writeProjectConfigTestFile(t, filepath.Join(projectDir, ".restish.json"), fmt.Sprintf(`{
  "apis": {
    "svc": {
      "base_url": %q,
      "profiles": {
        "default": {"auth_ref": "shared"}
      }
    }
  }
}`, server.URL))
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}
	c, _, _ = newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "get", "svc/anything", "-f", "body", "-o", "json"}); err != nil {
		t.Fatalf("project API request: %v", err)
	}
	if authHeader != "Bearer global-token" {
		t.Fatalf("Authorization = %q, want Bearer global-token", authHeader)
	}
}

func TestProjectConfigRelativeSpecFilesGenerateCommandsInProjectNamespace(t *testing.T) {
	_, cacheDir, projectDir := setupProjectConfigTest(t)
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	writeProjectConfigTestFile(t, filepath.Join(projectDir, "openapi.yaml"), fmt.Sprintf(`openapi: "3.1.0"
info:
  title: Project API
  version: "1.0"
servers:
  - url: %s
paths:
  /ping:
    get:
      operationId: getPing
      responses:
        "200":
          description: OK
`, server.URL))
	writeProjectConfigTestFile(t, filepath.Join(projectDir, ".restish.json"), fmt.Sprintf(`{
  "apis": {
    "svc": {
      "base_url": %q,
      "spec_files": ["openapi.yaml"]
    }
  }
}`, server.URL))
	t.Chdir(projectDir)

	c, _, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "config", "trust"}); err != nil {
		t.Fatalf("config trust: %v", err)
	}

	c, out, _ := newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "svc", "get-ping", "-f", "body", "-o", "json"}); err != nil {
		t.Fatalf("generated command: %v", err)
	}
	if requestedPath != "/ping" {
		t.Fatalf("requested path = %q, want /ping", requestedPath)
	}
	var body map[string]any
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatalf("decode output %q: %v", out.String(), err)
	}
	if body["ok"] != true {
		t.Fatalf("output body = %#v, want ok true", body)
	}

	specFiles, err := filepath.Glob(filepath.Join(cacheDir, "specs", "project-*-svc.cbor"))
	if err != nil {
		t.Fatalf("glob spec cache: %v", err)
	}
	if len(specFiles) != 1 {
		t.Fatalf("project spec cache files = %v, want one namespaced cache file", specFiles)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "specs", "svc.cbor")); !os.IsNotExist(err) {
		t.Fatalf("global spec cache exists for project API: %v", err)
	}

	c, out, _ = newProjectConfigTestCLI(t)
	if err := c.Run([]string{"restish", "svc", "--help"}); err != nil {
		t.Fatalf("project API help: %v", err)
	}
	if !strings.Contains(out.String(), "get-ping") {
		t.Fatalf("project API help = %q, want generated command from trusted project config", out.String())
	}

	c, out, _ = newProjectConfigTestCLI(t)
	_ = c.Run([]string{"restish", "__complete", "svc", ""})
	if !strings.Contains(out.String(), "get-ping") {
		t.Fatalf("project API completion = %q, want generated command from trusted project config", out.String())
	}
}
