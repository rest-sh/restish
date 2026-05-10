package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/spec"
)

// TestAPIConnectBuiltinNameRejected verifies that "api connect" refuses names that
// collide with top-level built-in commands (e.g. "api", "get", "post").
func TestAPIConnectBuiltinNameRejected(t *testing.T) {
	for _, name := range []string{"api", "get", "post", "cache", "edit"} {
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

// TestIsBuiltinCommandName verifies the helper covers the expected set of names.
func TestIsBuiltinCommandName(t *testing.T) {
	builtins := []string{"api", "cache", "cert", "completion", "config", "content-types", "delete", "doctor", "edit", "flags", "get", "head", "help", "links", "options", "patch", "plugin", "post", "put", "shell", "version"}
	for _, name := range builtins {
		if !isBuiltinCommandName(name) {
			t.Errorf("isBuiltinCommandName(%q) = false, want true", name)
		}
	}
	if isBuiltinCommandName("myapi") {
		t.Error("isBuiltinCommandName(\"myapi\") = true, want false")
	}
}

func TestBuiltinCommandNamesMatchRegisteredCommands(t *testing.T) {
	c := New()
	root := c.newRootCmd()
	for _, cmd := range root.Commands() {
		if cmd.Hidden {
			continue
		}
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
