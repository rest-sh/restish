package spec

import (
	"testing"
	"time"
)

const testSpecRaw = `{"openapi":"3.1.0","info":{"title":"Test","version":"1.0.0"},"paths":{}}`

func TestWriteAndReadCache(t *testing.T) {
	dir := t.TempDir()
	entry := &cacheEntry{
		Version:     "v2",
		FetchedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(time.Hour),
		ContentType: "application/json",
		Raw:         []byte(testSpecRaw),
	}
	if err := writeCache(dir, "testapi", entry); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	got, ok := readCache(dir, "testapi", "v2")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(got.Raw) != testSpecRaw {
		t.Errorf("Raw mismatch: got %q, want %q", got.Raw, testSpecRaw)
	}
}

func TestReadCache_Miss_Missing(t *testing.T) {
	_, ok := readCache(t.TempDir(), "nonexistent", "v2")
	if ok {
		t.Error("expected cache miss for nonexistent entry")
	}
}

func TestReadCache_Miss_VersionMismatch(t *testing.T) {
	dir := t.TempDir()
	entry := &cacheEntry{
		Version:     "v1",
		ExpiresAt:   time.Now().Add(time.Hour),
		ContentType: "application/json",
		Raw:         []byte(testSpecRaw),
	}
	writeCache(dir, "testapi", entry)

	_, ok := readCache(dir, "testapi", "v2")
	if ok {
		t.Error("expected cache miss for version mismatch")
	}
}

func TestReadCache_Miss_Expired(t *testing.T) {
	dir := t.TempDir()
	entry := &cacheEntry{
		Version:     "v2",
		ExpiresAt:   time.Now().Add(-time.Hour), // already expired
		ContentType: "application/json",
		Raw:         []byte(testSpecRaw),
	}
	writeCache(dir, "testapi", entry)

	_, ok := readCache(dir, "testapi", "v2")
	if ok {
		t.Error("expected cache miss for expired entry")
	}
}

func TestLoadFromCache(t *testing.T) {
	dir := t.TempDir()
	entry := &cacheEntry{
		Version:     "v2",
		ExpiresAt:   time.Now().Add(time.Hour),
		ContentType: "application/json",
		Raw:         []byte(testSpecRaw),
	}
	writeCache(dir, "testapi", entry)

	spec, err := LoadFromCache(dir, "testapi", "v2", DefaultLoaders())
	if err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}
	if spec == nil {
		t.Fatal("expected non-nil spec")
	}
}

func TestLoadFromCache_Miss(t *testing.T) {
	spec, err := LoadFromCache(t.TempDir(), "nonexistent", "v2", DefaultLoaders())
	if err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}
	if spec != nil {
		t.Error("expected nil spec for cache miss")
	}
}

func TestInvalidateCache(t *testing.T) {
	dir := t.TempDir()
	entry := &cacheEntry{
		Version:     "v2",
		ExpiresAt:   time.Now().Add(time.Hour),
		ContentType: "application/json",
		Raw:         []byte(testSpecRaw),
	}
	writeCache(dir, "testapi", entry)

	if err := InvalidateCache(dir, "testapi"); err != nil {
		t.Fatalf("InvalidateCache: %v", err)
	}

	_, ok := readCache(dir, "testapi", "v2")
	if ok {
		t.Error("expected cache miss after invalidation")
	}
}

func TestInvalidateCache_Nonexistent(t *testing.T) {
	// Should not error if the cache file doesn't exist.
	if err := InvalidateCache(t.TempDir(), "nonexistent"); err != nil {
		t.Fatalf("InvalidateCache: %v", err)
	}
}
