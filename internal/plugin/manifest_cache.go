package plugin

import (
	"fmt"
	"io"
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

// saveManifestCache writes cache to cachePath as CBOR. If the write fails a
// one-line warning is printed to w (a missing or stale cache is not fatal).
// Pass nil for w to suppress the warning.
func saveManifestCache(cachePath string, cache manifestCache, w io.Writer) {
	if cachePath == "" {
		return
	}
	data, err := cbor.Marshal(cache)
	if err != nil {
		if w != nil {
			fmt.Fprintf(w, "warning: manifest cache: marshal: %v\n", err)
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o700); err != nil {
		if w != nil {
			fmt.Fprintf(w, "warning: manifest cache: mkdir: %v\n", err)
		}
		return
	}
	if err := os.WriteFile(cachePath, data, 0o600); err != nil {
		if w != nil {
			fmt.Fprintf(w, "warning: manifest cache: write: %v\n", err)
		}
	}
}
