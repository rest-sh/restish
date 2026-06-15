package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAndSaveTokenCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.cbor")

	want := map[string]CachedToken{
		"billing:default": {
			AccessToken:  "abc",
			TokenType:    "Bearer",
			RefreshToken: "xyz",
			Expiry:       time.Now().Add(time.Hour).Round(time.Second),
		},
	}
	if err := SaveTokenCache(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadTokenCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 token, got %d", len(got))
	}
	if got["billing:default"].AccessToken != "abc" {
		t.Errorf("access_token = %q", got["billing:default"].AccessToken)
	}
	if !got["billing:default"].Expiry.Equal(want["billing:default"].Expiry) {
		t.Errorf("expiry = %v, want %v", got["billing:default"].Expiry, want["billing:default"].Expiry)
	}
}

func TestLoadTokenCacheMissing(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadTokenCache(filepath.Join(dir, "does-not-exist.cbor"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestDefaultTokenCachePathHonoursEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", dir)
	got := DefaultTokenCachePath()
	want := filepath.Join(dir, "tokens.cbor")
	if got != want {
		t.Errorf("DefaultTokenCachePath = %q, want %q", got, want)
	}
	// Sanity-check that the resolved path is actually under the override.
	if _, err := os.Stat(filepath.Dir(got)); err != nil {
		t.Errorf("config dir not created: %v", err)
	}
}
