package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

// TestAPIConnectBuiltinNameRejected verifies that "api connect" refuses names that
// collide with top-level built-in commands, including hidden compatibility commands.
func TestAPIConnectBuiltinNameRejected(t *testing.T) {
	for _, name := range []string{"api", "get", "post", "cache", "edit", "completion", "help"} {
		var stdout, stderr bytes.Buffer
		c := New()
		c.Stdout = &stdout
		c.Stderr = &stderr
		c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
		err := c.Run([]string{"restish", "api", "connect", name, "https://example.com", "--no-discover"})
		if err == nil {
			t.Errorf("api connect %q: expected error, got nil", name)
			continue
		}
		if !strings.Contains(err.Error(), "conflicts with a built-in command") {
			t.Errorf("api connect %q: unexpected error: %v", name, err)
		}
	}
}

func TestAPIConnectFlagsNameAllowed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := New()
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")

	if err := c.Run([]string{"restish", "api", "connect", "flags", "https://example.com", "--no-discover"}); err != nil {
		t.Fatalf("api connect flags: %v", err)
	}
	if !strings.Contains(stdout.String(), `Connected API "flags"`) {
		t.Fatalf("expected flags API to connect, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

// TestIsBuiltinCommandName verifies the helper covers the expected set of names.
func TestIsBuiltinCommandName(t *testing.T) {
	builtins := []string{"api", "cache", "cert", "completion", "config", "content-types", "delete", "doctor", "edit", "get", "head", "help", "links", "options", "patch", "plugin", "post", "put", "shell", "version"}
	for _, name := range builtins {
		if !isBuiltinCommandName(name) {
			t.Errorf("isBuiltinCommandName(%q) = false, want true", name)
		}
	}
	if isBuiltinCommandName("myapi") {
		t.Error("isBuiltinCommandName(\"myapi\") = true, want false")
	}
	if isBuiltinCommandName("flags") {
		t.Error("isBuiltinCommandName(\"flags\") = true, want false")
	}
}

func TestBuiltinCommandNamesMatchRegisteredCommands(t *testing.T) {
	c := New()
	root := c.newRootCmd()
	for _, cmd := range root.Commands() {
		if !isBuiltinCommandName(cmd.Name()) {
			t.Fatalf("registered built-in %q missing from builtinCommands", cmd.Name())
		}
	}
}

func TestXCLIPromptLooksSecretCommonNames(t *testing.T) {
	for _, name := range []string{
		"auth_token",
		"access_key",
		"credential",
		"credentials",
		"passphrase",
		"bearer",
		"private_key",
	} {
		if !xcliPromptLooksSecret(name) {
			t.Fatalf("xcliPromptLooksSecret(%q) = false, want true", name)
		}
	}
	if xcliPromptLooksSecret("organization") {
		t.Fatal("organization should not look secret")
	}
}

func TestReadXCLIPromptUsesSecretForSecretLookingName(t *testing.T) {
	c := &CLI{Stderr: io.Discard}
	var secretPrompts int
	c.hooks.PromptFunc = func(context.Context, string) (string, error) {
		t.Fatal("expected secret prompt path")
		return "", nil
	}
	c.hooks.SecretFunc = func(context.Context, string) (string, error) {
		secretPrompts++
		return "secret-value", nil
	}

	got, err := c.readXCLIPrompt(context.Background(), "default", "auth_token", spec.XCLIPromptVar{})
	if err != nil {
		t.Fatalf("readXCLIPrompt: %v", err)
	}
	if got != "secret-value" {
		t.Fatalf("value = %q, want secret-value", got)
	}
	if secretPrompts != 1 {
		t.Fatalf("secret prompts = %d, want 1", secretPrompts)
	}
}

func TestAPIConnectFallbackOAuthNoninteractiveReportsSetupExpression(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
  "openapi": "3.1.0",
  "info": {"title": "Box-like API", "version": "1.0"},
  "components": {
    "securitySchemes": {
      "OAuth2Security": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {
            "authorizationUrl": "https://auth.example.com/authorize",
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {}
          }
        }
      }
    }
  },
  "security": [{"OAuth2Security": []}],
  "paths": {}
}`), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	oldOpenTTY := promptOpenTTY
	promptOpenTTY = func() (*os.File, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { promptOpenTTY = oldOpenTTY })

	var stdout, stderr bytes.Buffer
	c := New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	c.Hooks().SpecCachePath = t.TempDir()

	err := c.Run([]string{"restish", "api", "connect", "box", "https://api.example.com", "--spec", specPath})
	if err == nil {
		t.Fatal("expected noninteractive OAuth setup to fail with setup-expression guidance")
	}
	if !strings.Contains(err.Error(), "missing required auth setup value") {
		t.Fatalf("expected missing setup diagnostic, got: %v", err)
	}
	if !strings.Contains(err.Error(), "prompt.credentials.OAuth2Security.client_id:<value>") {
		t.Fatalf("expected credential setup expression, got: %v", err)
	}
	if strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("expected direct setup diagnostic, got: %v", err)
	}
}

func TestAPIConnectFallbackOAuthNoninteractiveUsesDefaultScopes(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
  "openapi": "3.1.0",
  "info": {"title": "OAuth Test", "version": "1.0"},
  "servers": [{"url": "https://api.example.com"}],
  "components": {
    "securitySchemes": {
      "OAuth": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {
            "authorizationUrl": "https://auth.example.com/authorize",
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {
              "read:profile": "Read profile",
              "read:recovery": "Read recovery"
            }
          }
        }
      }
    }
  },
  "paths": {
    "/recovery": {
      "get": {
        "operationId": "getRecovery",
        "security": [{"OAuth": ["read:profile", "read:recovery"]}],
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	oldOpenTTY := promptOpenTTY
	promptOpenTTY = func() (*os.File, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { promptOpenTTY = oldOpenTTY })

	var stdout, stderr bytes.Buffer
	c := New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	c.Hooks().SpecCachePath = t.TempDir()

	err := c.Run([]string{
		"restish", "api", "connect", "whoop", "https://api.example.com",
		"--spec", specPath,
		"--yes",
		"prompt.credentials.OAuth.client_id:codex-dummy",
	})
	if err != nil {
		t.Fatalf("api connect: %v\nstderr:\n%s", err, stderr.String())
	}
	if strings.Contains(stderr.String(), "Scopes [") {
		t.Fatalf("noninteractive connect prompted for scopes:\n%s", stderr.String())
	}

	written, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	credential := written.APIs["whoop"].Profiles["default"].Credentials["OAuth"]
	if credential == nil || credential.Auth == nil {
		t.Fatalf("OAuth credential missing: %#v", written.APIs["whoop"].Profiles["default"].Credentials)
	}
	if got := credential.Auth.Params["client_id"]; got != "codex-dummy" {
		t.Fatalf("client_id = %q, want codex-dummy", got)
	}
	if got := credential.Auth.Params["scopes"]; got != "read:profile read:recovery" {
		t.Fatalf("scopes = %q, want read:profile read:recovery", got)
	}
}

func TestAPIConnectFallbackOAuthUsesExplicitScopesForSatisfies(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
  "openapi": "3.1.0",
  "info": {"title": "OAuth Test", "version": "1.0"},
  "servers": [{"url": "https://api.example.com"}],
  "components": {
    "securitySchemes": {
      "OAuth": {
        "type": "oauth2",
        "flows": {
          "authorizationCode": {
            "authorizationUrl": "https://auth.example.com/authorize",
            "tokenUrl": "https://auth.example.com/token",
            "scopes": {
              "read:profile": "Read profile",
              "read:recovery": "Read recovery"
            }
          }
        }
      }
    }
  },
  "paths": {
    "/profile": {"get": {"operationId": "getProfile", "security": [{"OAuth": ["read:profile"]}], "responses": {"200": {"description": "OK"}}}},
    "/recovery": {"get": {"operationId": "getRecovery", "security": [{"OAuth": ["read:recovery"]}], "responses": {"200": {"description": "OK"}}}}
  }
}`), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	oldOpenTTY := promptOpenTTY
	promptOpenTTY = func() (*os.File, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() { promptOpenTTY = oldOpenTTY })

	var stdout, stderr bytes.Buffer
	c := New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	c.Hooks().SpecCachePath = t.TempDir()

	err := c.Run([]string{
		"restish", "api", "connect", "whoop", "https://api.example.com",
		"--spec", specPath,
		"--yes",
		"prompt.credentials.OAuth.client_id:codex-dummy",
		"prompt.credentials.OAuth.scopes:read:profile",
	})
	if err != nil {
		t.Fatalf("api connect: %v\nstderr:\n%s", err, stderr.String())
	}

	written, err := config.Load(c.Hooks().ConfigPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	credential := written.APIs["whoop"].Profiles["default"].Credentials["OAuth"]
	if credential == nil || credential.Auth == nil {
		t.Fatalf("OAuth credential missing: %#v", written.APIs["whoop"].Profiles["default"].Credentials)
	}
	if got := credential.Auth.Params["scopes"]; got != "read:profile" {
		t.Fatalf("scopes = %q, want read:profile", got)
	}
	if len(credential.Satisfies) != 1 || credential.Satisfies[0] != "read:profile" {
		t.Fatalf("satisfies = %#v, want [read:profile]", credential.Satisfies)
	}
}
