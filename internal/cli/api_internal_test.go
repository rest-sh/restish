package cli

import (
	"reflect"
	"testing"
)

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
