package cli_test

import (
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
)

func TestAPIAuthListAddRemove(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {}
      }
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "add", "myapi", "PartnerKey"}); err != nil {
		t.Fatalf("api auth add: %v", err)
	}
	written, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if written.APIs["myapi"].Profiles["default"].Credentials["PartnerKey"] == nil {
		t.Fatal("expected PartnerKey credential")
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "list", "myapi"}); err != nil {
		t.Fatalf("api auth list: %v", err)
	}
	if !strings.Contains(out.String(), "PartnerKey") {
		t.Fatalf("expected PartnerKey in list output, got %q", out.String())
	}

	if err := c.Run([]string{"restish", "api", "auth", "remove", "myapi", "PartnerKey"}); err != nil {
		t.Fatalf("api auth remove: %v", err)
	}
	written, err = config.Load(cfgFile)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if written.APIs["myapi"].Profiles["default"].Credentials != nil {
		t.Fatalf("expected credentials removed, got %#v", written.APIs["myapi"].Profiles["default"].Credentials)
	}
}

func TestAPIAuthInspectCredentialAPIKey(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {
              "auth": {
                "type": "api-key",
                "params": {"in": "header", "name": "X-Partner-Key", "value": "secret"}
              }
            }
          }
        }
      }
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--rsh-credential", "PartnerKey"}); err != nil {
		t.Fatalf("api auth inspect: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "secret") || !strings.Contains(got, "X-Partner-Key: <redacted>") {
		t.Fatalf("expected redacted API key inspection, got %q", got)
	}
}

func TestAPIAuthInspectSingleCredentialByDefault(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {
              "auth": {
                "type": "api-key",
                "params": {"in": "header", "name": "X-Partner-Key", "value": "secret"}
              }
            }
          }
        }
      }
    }
  }
}`)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi"}); err != nil {
		t.Fatalf("api auth inspect: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Credential: PartnerKey") {
		t.Fatalf("expected selected credential in output, got %q", got)
	}
	if strings.Contains(got, "secret") || !strings.Contains(got, "X-Partner-Key: <redacted>") {
		t.Fatalf("expected redacted API key inspection, got %q", got)
	}

	out.Reset()
	if err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi", "--raw-header", "X-Partner-Key"}); err != nil {
		t.Fatalf("api auth inspect raw header: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "secret" {
		t.Fatalf("raw header = %q, want secret", got)
	}
}

func TestAPIAuthInspectMultipleCredentialsRequiresSelection(t *testing.T) {
	cfgFile := writeAPIConfig(t, `{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "credentials": {
            "PartnerKey": {
              "auth": {
                "type": "api-key",
                "params": {"in": "header", "name": "X-Partner-Key", "value": "secret"}
              }
            },
            "UserBearer": {
              "auth": {
                "type": "bearer",
                "params": {"token": "user-token"}
              }
            }
          }
        }
      }
    }
  }
}`)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	err := c.Run([]string{"restish", "api", "auth", "inspect", "myapi"})
	if err == nil {
		t.Fatal("expected multiple credentials error")
	}
	got := err.Error()
	for _, want := range []string{"multiple configured credentials", "PartnerKey, UserBearer", "--rsh-credential"} {
		if !strings.Contains(got, want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}
