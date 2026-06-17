package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigFileHasInsecurePermissionsWindowsExistingFileUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restish.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	insecure, err := configFileHasInsecurePermissions(path, "windows")
	if insecure {
		t.Fatal("expected unsupported Windows check not to report insecure")
	}
	if !errors.Is(err, ErrPermissionCheckUnsupported) {
		t.Fatalf("expected unsupported permission check error, got %v", err)
	}
}

func TestConfigFileHasInsecurePermissionsWindowsMissingFileOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-restish.json")

	insecure, err := configFileHasInsecurePermissions(path, "windows")
	if err != nil {
		t.Fatalf("expected missing file to remain ok, got %v", err)
	}
	if insecure {
		t.Fatal("expected missing file not to report insecure")
	}
}

func TestClosestJSONFieldScalesWithLengthAndRunes(t *testing.T) {
	fields := jsonFieldTypes(reflect.TypeOf(APIConfig{}))
	if got := closestJSONField("allow_cross_origin_specs", fields); got != "allow_cross_origin_spec" {
		t.Fatalf("long suggestion = %q, want allow_cross_origin_spec", got)
	}
	if got := closestJSONField("x", fields); got != "" {
		t.Fatalf("short suggestion = %q, want none", got)
	}
	if got := levenshteinDistance("café", "cafe"); got != 1 {
		t.Fatalf("rune levenshtein = %d, want 1", got)
	}
}
