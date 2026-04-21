package cli

import (
	"reflect"
	"strings"
	"testing"
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
	builtins := []string{"api", "cache", "cert", "completion", "delete", "edit", "get", "head", "help", "links", "options", "patch", "plugin", "post", "put", "setup"}
	for _, name := range builtins {
		if !isBuiltinCommandName(name) {
			t.Errorf("isBuiltinCommandName(%q) = false, want true", name)
		}
	}
	if isBuiltinCommandName("myapi") {
		t.Error("isBuiltinCommandName(\"myapi\") = true, want false")
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
