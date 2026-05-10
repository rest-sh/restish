package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
)

func TestConfigCommandPathShowAndSet(t *testing.T) {
	cfg := `{
  "auth_profiles": {
    "shared": {
      "type": "oauth-client-credentials",
      "params": {
        "client_id": "docs",
        "client_secret": "super-secret"
      }
    }
  },
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "headers": [
            "Authorization: Bearer profile-secret",
            "X-Trace: visible"
          ],
          "query": [
            "api_key=query-secret",
            "page=1"
          ],
          "auth": {
            "type": "api-key",
            "params": {
              "in": "header",
              "name": "X-API-Key",
              "value": "docs-key"
            }
          }
        }
      }
    }
  }
}`
	c, out, _ := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := c.Run([]string{"restish", "config", "path"}); err != nil {
		t.Fatalf("config path: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != c.Hooks().ConfigPath {
		t.Fatalf("config path = %q, want %q", got, c.Hooks().ConfigPath)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "config", "show"}); err != nil {
		t.Fatalf("config show: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "APIs: 1") || !strings.Contains(got, "Auth profiles: 1") {
		t.Fatalf("unexpected config summary:\n%s", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "config", "show", "--json"}); err != nil {
		t.Fatalf("config show --json: %v", err)
	}
	if got := out.String(); strings.Contains(got, "super-secret") || strings.Contains(got, "docs-key") || !strings.Contains(got, `"client_secret": "***"`) || !strings.Contains(got, `"value": "***"`) {
		t.Fatalf("config show --json did not redact secret:\n%s", got)
	}
	if got := out.String(); strings.Contains(got, "profile-secret") || strings.Contains(got, "query-secret") ||
		!strings.Contains(got, `"Authorization: ***"`) ||
		!strings.Contains(got, `"api_key=***"`) ||
		!strings.Contains(got, `"X-Trace: visible"`) ||
		!strings.Contains(got, `"page=1"`) {
		t.Fatalf("config show --json did not redact persistent request credentials correctly:\n%s", got)
	}

	if err := c.Run([]string{"restish", "config", "set", "cache.max_size: 250MB"}); err != nil {
		t.Fatalf("config set: %v", err)
	}
	loaded, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := loaded.Cache.MaxSize; got != "250MB" {
		t.Fatalf("cache.max_size = %q, want 250MB", got)
	}
}

func TestConfigSetFullAuthObject(t *testing.T) {
	cfg := `{
  "apis": {
    "example": {
      "base_url": "https://api.example.com"
    }
  }
}`
	c, _, _ := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := c.Run([]string{
		"restish", "config", "set",
		`apis.example.profiles.demo.auth: {type: http-basic, params: {username: demo, password: env:DEMO_PASSWORD}}`,
	}); err != nil {
		t.Fatalf("config set full auth object: %v", err)
	}

	loaded, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	auth := loaded.APIs["example"].Profiles["demo"].Auth
	if auth == nil || auth.Type != "http-basic" || auth.Params["username"] != "demo" || auth.Params["password"] != "env:DEMO_PASSWORD" {
		t.Fatalf("auth = %#v", auth)
	}
}

func TestConfigSetRejectsNonPatchForm(t *testing.T) {
	c, _, _ := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	err := c.Run([]string{"restish", "config", "set", "cache.max_size", "250MB"})
	if err == nil {
		t.Fatal("expected non-patch form to be rejected")
	}
	if !strings.Contains(err.Error(), "expected shorthand patch expression") {
		t.Fatalf("unexpected error: %v", err)
	}
}
