package plugin

import (
	"os"
	"path/filepath"

	"github.com/fxamacker/cbor/v2"
)

// manifestCacheEntry stores one plugin's cached manifest along with the
// modification time of the executable at the time it was cached.
type manifestCacheEntry struct {
	Mtime    int64    `cbor:"mtime"`
	Manifest Manifest `cbor:"manifest"`
}

// manifestCache maps plugin executable path to its cached entry.
type manifestCache map[string]manifestCacheEntry

// loadManifestCache reads the manifest cache CBOR file at cachePath.
// Returns an empty map on any error (missing file, corrupt data, etc.).
func loadManifestCache(cachePath string) manifestCache {
	if cachePath == "" {
		return make(manifestCache)
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return make(manifestCache)
	}
	var cache manifestCache
	if err := cbor.Unmarshal(data, &cache); err != nil {
		return make(manifestCache)
	}
	return cache
}

// saveManifestCache writes cache to cachePath as CBOR.
// Errors are silently ignored; a stale or missing cache is not fatal.
func saveManifestCache(cachePath string, cache manifestCache) {
	if cachePath == "" {
		return
	}
	data, err := cbor.Marshal(cache)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(cachePath, data, 0o600)
}
