package filter

import (
	"container/list"
	"fmt"
	"testing"
)

func resetJQCacheForTest(t *testing.T) {
	t.Helper()
	jqCacheMu.Lock()
	t.Cleanup(func() {
		jqCacheMu.Lock()
		jqCache = make(map[string]*list.Element, jqCacheMaxSize)
		jqLRU.Init()
		jqCacheMu.Unlock()
	})
	jqCache = make(map[string]*list.Element, jqCacheMaxSize)
	jqLRU.Init()
	jqCacheMu.Unlock()
}

func TestCompiledJQCacheRemainsBounded(t *testing.T) {
	resetJQCacheForTest(t)

	for i := range jqCacheMaxSize + 50 {
		if _, err := compiledJQ(fmt.Sprintf(".body.items[%d]?", i)); err != nil {
			t.Fatalf("compiledJQ failed: %v", err)
		}
	}

	jqCacheMu.Lock()
	defer jqCacheMu.Unlock()
	if len(jqCache) > jqCacheMaxSize {
		t.Fatalf("cache size = %d, want at most %d", len(jqCache), jqCacheMaxSize)
	}
	if jqLRU.Len() != len(jqCache) {
		t.Fatalf("lru length = %d, cache length = %d", jqLRU.Len(), len(jqCache))
	}
}

func TestCompiledJQCacheKeepsHotExpression(t *testing.T) {
	resetJQCacheForTest(t)

	const hot = ".body.name"
	if _, err := compiledJQ(hot); err != nil {
		t.Fatalf("compiled hot expression: %v", err)
	}
	for i := range jqCacheMaxSize - 1 {
		if _, err := compiledJQ(fmt.Sprintf(".body.items[%d]?", i)); err != nil {
			t.Fatalf("compiledJQ failed: %v", err)
		}
	}
	if _, err := compiledJQ(hot); err != nil {
		t.Fatalf("recompiled hot expression: %v", err)
	}
	for i := range 50 {
		if _, err := compiledJQ(fmt.Sprintf(".headers[\"x-%d\"]?", i)); err != nil {
			t.Fatalf("compiledJQ failed: %v", err)
		}
	}

	jqCacheMu.Lock()
	defer jqCacheMu.Unlock()
	if _, ok := jqCache[hot]; !ok {
		t.Fatal("hot expression was evicted")
	}
	if len(jqCache) > jqCacheMaxSize {
		t.Fatalf("cache size = %d, want at most %d", len(jqCache), jqCacheMaxSize)
	}
}
