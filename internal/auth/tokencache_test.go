package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenCache_MissingFile(t *testing.T) {
	tc := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	got, err := tc.Get("key")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil token for missing file, got %+v", got)
	}
}

func TestTokenCache_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	tc := NewTokenCache(path)
	tok := CachedToken{
		AccessToken:  "abc123",
		TokenType:    "bearer",
		RefreshToken: "refresh",
		Expiry:       time.Now().Add(time.Hour).Truncate(time.Second),
	}
	if err := tc.Set("mykey", tok); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := tc.Get("mykey")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected token, got nil")
	}
	if got.AccessToken != tok.AccessToken {
		t.Errorf("AccessToken: got %q, want %q", got.AccessToken, tok.AccessToken)
	}
	if got.RefreshToken != tok.RefreshToken {
		t.Errorf("RefreshToken: got %q, want %q", got.RefreshToken, tok.RefreshToken)
	}
}

func TestTokenCache_Delete(t *testing.T) {
	tc := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	_ = tc.Set("key", CachedToken{AccessToken: "token"})
	if err := tc.Delete("key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := tc.Get("key")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func TestTokenCache_Delete_Missing(t *testing.T) {
	tc := NewTokenCache(filepath.Join(t.TempDir(), "tokens.json"))
	if err := tc.Delete("no-such-key"); err != nil {
		t.Errorf("Delete of absent key should return nil, got %v", err)
	}
}

func TestTokenCache_FilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	tc := NewTokenCache(path)
	if err := tc.Set("k", CachedToken{AccessToken: "x"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected file permission 0600, got %04o", perm)
	}
}

func TestTokenCache_DirPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")
	path := filepath.Join(dir, "tokens.json")
	tc := NewTokenCache(path)
	if err := tc.Set("k", CachedToken{AccessToken: "x"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("expected dir permission 0700, got %04o", perm)
	}
}

func TestCachedToken_IsExpired(t *testing.T) {
	past := CachedToken{Expiry: time.Now().Add(-time.Hour)}
	if !past.IsExpired() {
		t.Error("expected expired token to be IsExpired")
	}
	future := CachedToken{Expiry: time.Now().Add(time.Hour)}
	if future.IsExpired() {
		t.Error("expected unexpired token to not be IsExpired")
	}
	zero := CachedToken{}
	if zero.IsExpired() {
		t.Error("token with zero Expiry should not be IsExpired")
	}
}
