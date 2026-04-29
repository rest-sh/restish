package cli

import (
	"context"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/spec"
)

// TestAPIAddBuiltinNameRejected verifies that "api add" refuses names that
// collide with top-level built-in commands (e.g. "api", "get", "post").
func TestAPIAddBuiltinNameRejected(t *testing.T) {
	for _, name := range []string{"api", "get", "post", "cache", "edit"} {
		c := &CLI{}
		err := c.runAPIAdd(nil, []string{name, "https://example.com"})
		if err == nil {
			t.Errorf("runAPIAdd(%q): expected error, got nil", name)
			continue
		}
		if !strings.Contains(err.Error(), "conflicts with a built-in command") {
			t.Errorf("runAPIAdd(%q): unexpected error: %v", name, err)
		}
	}
}

// TestIsBuiltinCommandName verifies the helper covers the expected set of names.
func TestIsBuiltinCommandName(t *testing.T) {
	builtins := []string{"api", "cache", "cert", "completion", "delete", "edit", "get", "head", "help", "links", "options", "patch", "plugin", "post", "put", "setup", "theme"}
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

func TestResolveAPIConfigKey_ProfileAuthParam(t *testing.T) {
	got, err := resolveAPIConfigKey("myapi", "profiles.default.auth.params.token")
	if err != nil {
		t.Fatalf("resolveAPIConfigKey: %v", err)
	}
	want := []string{"apis", "myapi", "profiles", "default", "auth", "params", "token"}
	if got.kind != apiKeyProfileAuthParam {
		t.Fatalf("kind = %v, want %v", got.kind, apiKeyProfileAuthParam)
	}
	if got.profileName != "default" {
		t.Fatalf("profileName = %q, want %q", got.profileName, "default")
	}
	if got.paramName != "token" {
		t.Fatalf("paramName = %q, want %q", got.paramName, "token")
	}
	if !reflect.DeepEqual(got.jsonPath, want) {
		t.Fatalf("jsonPath = %#v, want %#v", got.jsonPath, want)
	}
}
