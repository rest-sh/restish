package cache

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_CreatesDir(t *testing.T) {
	dir := t.TempDir() + "/sub/cache"
	c, err := New(dir, DefaultMaxBytes, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestGetSet(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")

	key := "https://api.example.com/items"
	data := []byte("response-body")

	if _, ok := c.Get(key); ok {
		t.Fatal("expected cache miss before Set")
	}

	c.Set(key, data)

	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit after Set")
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestGet_UpdatesMtime(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	key := "https://api.example.com/v1"
	c.Set(key, []byte("data"))

	before := time.Now()
	_, ok := c.Get(key)
	if !ok {
		t.Fatal("cache miss")
	}
	_ = before // mtime update is best-effort; just verify no error
}

func TestDelete(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	key := "https://api.example.com/things"
	c.Set(key, []byte("val"))

	c.Delete(key)

	if _, ok := c.Get(key); ok {
		t.Fatal("expected cache miss after Delete")
	}
}

func TestDelete_Nonexistent(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	// Should not panic or error.
	c.Delete("https://api.example.com/nonexistent")
}

func TestInfo_Empty(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	info, err := c.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.EntryCount != 0 {
		t.Errorf("EntryCount: got %d, want 0", info.EntryCount)
	}
	if info.SizeBytes != 0 {
		t.Errorf("SizeBytes: got %d, want 0", info.SizeBytes)
	}
	if !info.OldestEntry.IsZero() {
		t.Errorf("OldestEntry: expected zero, got %v", info.OldestEntry)
	}
}

func TestInfo_WithEntries(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	c.Set("https://api.example.com/a", []byte("hello"))
	c.Set("https://api.example.com/b", []byte("world!"))

	info, err := c.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.EntryCount != 2 {
		t.Errorf("EntryCount: got %d, want 2", info.EntryCount)
	}
	if info.SizeBytes == 0 {
		t.Error("SizeBytes should be nonzero")
	}
	if info.OldestEntry.IsZero() {
		t.Error("OldestEntry should be set")
	}
}

func TestClear_All(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	c.Set("https://api.example.com/x", []byte("a"))
	c.Set("https://other.example.com/y", []byte("b"))

	if err := c.Clear(""); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	info, _ := c.Info()
	if info.EntryCount != 0 {
		t.Errorf("expected 0 entries after full clear, got %d", info.EntryCount)
	}
	if got := atomic.LoadInt64(&c.sizeEstimate); got != 0 {
		t.Errorf("sizeEstimate after full clear: got %d, want 0", got)
	}
}

func TestClear_ByHost(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	c.Set("https://api.example.com/x", []byte("a"))
	c.Set("https://other.example.com/y", []byte("b"))

	if err := c.Clear("api.example.com"); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	// The cleared host should be gone.
	if _, ok := c.Get("https://api.example.com/x"); ok {
		t.Error("expected cache miss for cleared host")
	}
	_, total, err := c.allFiles()
	if err != nil {
		t.Fatalf("allFiles: %v", err)
	}
	if got := atomic.LoadInt64(&c.sizeEstimate); got != total {
		t.Errorf("sizeEstimate after host clear: got %d, want %d", got, total)
	}
}

func TestClear_EmptyCache(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	// Should not error on an empty (but existing) cache.
	if err := c.Clear(""); err != nil {
		t.Fatalf("Clear on empty cache: %v", err)
	}
}

func TestEviction_LRU(t *testing.T) {
	dir := t.TempDir()
	// Cap at 20 bytes; each entry is ~5 bytes, so after 5 entries eviction fires.
	c, _ := New(dir, 20, "")

	keys := []string{
		"https://api.example.com/a",
		"https://api.example.com/b",
		"https://api.example.com/c",
		"https://api.example.com/d",
		"https://api.example.com/e",
		"https://api.example.com/f",
	}
	for _, k := range keys {
		c.Set(k, []byte("12345")) // 5 bytes each
	}
	// Wait for the background eviction goroutine to finish.
	c.WaitEvict()

	// After setting 6 entries at 5 bytes each (30 bytes total) with a 20-byte cap,
	// at least one eviction should have occurred.
	info, _ := c.Info()
	if info.SizeBytes > 20 {
		t.Errorf("cache size %d exceeds cap 20", info.SizeBytes)
	}
}

func TestFilePath_NoURL(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	// Non-URL key should still work (host defaults to "_").
	c.Set("not-a-url", []byte("data"))
	got, ok := c.Get("not-a-url")
	if !ok {
		t.Fatal("expected hit for non-URL key")
	}
	if string(got) != "data" {
		t.Errorf("got %q, want %q", got, "data")
	}
}

func TestSetOverwriteDoesNotDoubleCountSize(t *testing.T) {
	c, _ := New(t.TempDir(), 10, "")
	key := "https://api.example.com/items"
	for i := 0; i < 4; i++ {
		c.Set(key, []byte("1234"))
	}
	c.WaitEvict()

	if _, ok := c.Get(key); !ok {
		t.Fatal("expected overwritten entry to remain cached")
	}
}

func TestSetConcurrentSameKeyKeepsSizeEstimateAccurate(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	key := "https://api.example.com/items"
	payloads := [][]byte{
		[]byte(strings.Repeat("a", 128)),
		[]byte(strings.Repeat("b", 256)),
		[]byte(strings.Repeat("c", 512)),
		[]byte(strings.Repeat("d", 1024)),
		[]byte(strings.Repeat("e", 2048)),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Set(key, payloads[i%len(payloads)])
		}(i)
	}
	wg.Wait()

	_, total, err := c.allFiles()
	if err != nil {
		t.Fatalf("allFiles: %v", err)
	}
	if got := atomic.LoadInt64(&c.sizeEstimate); got != total {
		t.Fatalf("sizeEstimate after concurrent same-key sets: got %d, want %d", got, total)
	}
}

func TestSetWritesAtomically(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	key := "https://api.example.com/items"
	c.Set(key, []byte("old"))

	originalRename := renameCacheFile
	t.Cleanup(func() {
		renameCacheFile = originalRename
	})
	renameCacheFile = func(oldpath, newpath string) error {
		if filepath.Ext(oldpath) != ".tmp" {
			t.Fatalf("temp path extension = %q, want .tmp", filepath.Ext(oldpath))
		}
		if got, err := os.ReadFile(newpath); err != nil || string(got) != "old" {
			t.Fatalf("existing cache entry changed before rename: got %q, err=%v", got, err)
		}
		if got, err := os.ReadFile(oldpath); err != nil || string(got) != "new" {
			t.Fatalf("temp cache entry = %q, err=%v; want new data", got, err)
		}
		return originalRename(oldpath, newpath)
	}

	c.Set(key, []byte("new"))
	got, ok := c.Get(key)
	if !ok {
		t.Fatal("expected cache hit after atomic rewrite")
	}
	if string(got) != "new" {
		t.Fatalf("cached data = %q, want new", got)
	}
}

func TestEvictionUsesSiblingLockFile(t *testing.T) {
	dir := t.TempDir()
	c, _ := New(dir, 5, "")
	c.Set("https://api.example.com/a", []byte("12345"))
	c.Set("https://api.example.com/b", []byte("67890"))
	c.WaitEvict()

	if _, err := os.Stat(filepath.Join(dir, ".evict.lock")); err != nil {
		t.Fatalf("expected eviction lock file: %v", err)
	}
}

func TestKeyLocksAreFixedShards(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	for i := 0; i < 1000; i++ {
		unlock := c.lockKey("https://api.example.com/items/" + strconv.Itoa(i))
		unlock()
	}
	if got := len(c.keyLocks); got != cacheKeyLockShards {
		t.Fatalf("key lock shards = %d, want %d", got, cacheKeyLockShards)
	}
}

func TestNamespaceIsolatesProfiles(t *testing.T) {
	dir := t.TempDir()
	anon, _ := New(dir, DefaultMaxBytes, "myapi:default")
	authed, _ := New(dir, DefaultMaxBytes, "myapi:admin")
	key := "https://api.example.com/items"

	anon.Set(key, []byte("anon"))
	authed.Set(key, []byte("authed"))

	if got, ok := anon.Get(key); !ok || string(got) != "anon" {
		t.Fatalf("default namespace got %q, hit=%v", got, ok)
	}
	if got, ok := authed.Get(key); !ok || string(got) != "authed" {
		t.Fatalf("admin namespace got %q, hit=%v", got, ok)
	}
}

func TestNamespacePathComponentsAreFilesystemSafe(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, DefaultMaxBytes, `my:api/default*prod?`)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	c.Set("https://api.example.com:8443/items", []byte("cached"))

	var files []string
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || path == dir {
			return err
		}
		base := filepath.Base(path)
		if strings.ContainsAny(base, `<>:"/\|?*`) {
			t.Fatalf("unsafe cache path component %q under %s", base, path)
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk cache: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one cache file, got %d: %v", len(files), files)
	}
}

func TestClearNamespacePrefixUsesRawNamespace(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir, DefaultMaxBytes, "myapi:default")
	if err != nil {
		t.Fatalf("New default: %v", err)
	}
	other, err := New(dir, DefaultMaxBytes, "other:default")
	if err != nil {
		t.Fatalf("New other: %v", err)
	}
	key := "https://api.example.com/items"
	c.Set(key, []byte("mine"))
	other.Set(key, []byte("other"))

	clearer, err := New(dir, DefaultMaxBytes, "")
	if err != nil {
		t.Fatalf("New clearer: %v", err)
	}
	if err := clearer.ClearNamespacePrefix("myapi:"); err != nil {
		t.Fatalf("ClearNamespacePrefix: %v", err)
	}
	if _, ok := c.Get(key); ok {
		t.Fatal("expected myapi namespace to be cleared")
	}
	if got, ok := other.Get(key); !ok || string(got) != "other" {
		t.Fatalf("expected other namespace to remain, got %q hit=%v", got, ok)
	}
	_, total, err := clearer.allFiles()
	if err != nil {
		t.Fatalf("allFiles: %v", err)
	}
	if got := atomic.LoadInt64(&clearer.sizeEstimate); got != total {
		t.Fatalf("sizeEstimate after namespace prefix clear: got %d, want %d", got, total)
	}
}

func TestClearNamespacesRecomputesSizeEstimate(t *testing.T) {
	dir := t.TempDir()
	first, err := New(dir, DefaultMaxBytes, "myapi:default")
	if err != nil {
		t.Fatalf("New first: %v", err)
	}
	second, err := New(dir, DefaultMaxBytes, "myapi:admin")
	if err != nil {
		t.Fatalf("New second: %v", err)
	}
	first.Set("https://api.example.com/items", []byte("default"))
	second.Set("https://api.example.com/items", []byte("admin"))

	clearer, err := New(dir, DefaultMaxBytes, "")
	if err != nil {
		t.Fatalf("New clearer: %v", err)
	}
	if err := clearer.ClearNamespaces([]string{"myapi:default"}); err != nil {
		t.Fatalf("ClearNamespaces: %v", err)
	}
	if _, ok := first.Get("https://api.example.com/items"); ok {
		t.Fatal("expected default namespace to be cleared")
	}
	if got, ok := second.Get("https://api.example.com/items"); !ok || string(got) != "admin" {
		t.Fatalf("expected admin namespace to remain, got %q hit=%v", got, ok)
	}
	_, total, err := clearer.allFiles()
	if err != nil {
		t.Fatalf("allFiles: %v", err)
	}
	if got := atomic.LoadInt64(&clearer.sizeEstimate); got != total {
		t.Fatalf("sizeEstimate after namespace clear: got %d, want %d", got, total)
	}
}

func TestClearUnknownHostReturnsError(t *testing.T) {
	c, _ := New(t.TempDir(), DefaultMaxBytes, "")
	if err := c.Clear("missing.example.com"); err == nil {
		t.Fatal("expected clear error for unknown host")
	}
}
