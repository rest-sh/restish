package config

import (
	"errors"
	"os"
	"path/filepath"
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
