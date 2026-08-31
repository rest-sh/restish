//go:build windows

package auth

import (
	"os"
	"path/filepath"
	"testing"
)

// On Windows ConfigFileHasInsecurePermissions returns ErrPermissionCheckUnsupported
// for every existing file. The cache must still read back what it wrote, or every
// request after the first login re-opens the browser.
func TestTokenCache_ReadsExistingFileOnWindows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.cbor")
	tc := NewTokenCache(path)
	if err := tc.Set("api:default", CachedToken{AccessToken: "abc"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}
	got, err := NewTokenCache(path).Get("api:default")
	if err != nil {
		t.Fatalf("Get on existing file: %v", err)
	}
	if got == nil || got.AccessToken != "abc" {
		t.Fatalf("Get = %#v, want access_token abc", got)
	}
}
