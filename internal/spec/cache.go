package spec

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// cacheEntry is the on-disk format for a cached spec.
type cacheEntry struct {
	Version     string    `cbor:"version"`
	FetchedAt   time.Time `cbor:"fetched_at"`
	ExpiresAt   time.Time `cbor:"expires_at"`
	ContentType string    `cbor:"content_type"`
	Raw         []byte    `cbor:"raw"`
}

// cacheFile returns the path of the CBOR cache file for the given API.
func cacheFile(cacheDir, apiName string) string {
	return filepath.Join(cacheDir, apiName+".cbor")
}

// readCache loads and validates a cached spec entry.
// Returns the entry and true if the cache is valid (not expired, version matches).
func readCache(cacheDir, apiName, version string) (*cacheEntry, bool) {
	path := cacheFile(cacheDir, apiName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var e cacheEntry
	if err := cbor.Unmarshal(data, &e); err != nil {
		return nil, false
	}
	if e.Version != version {
		return nil, false // restish version changed
	}
	if !e.ExpiresAt.IsZero() && time.Now().After(e.ExpiresAt) {
		return nil, false // TTL expired
	}
	return &e, true
}

// writeCache serialises entry to the CBOR cache file.
func writeCache(cacheDir, apiName string, entry *cacheEntry) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("spec cache: mkdir: %w", err)
	}
	data, err := cbor.Marshal(entry)
	if err != nil {
		return fmt.Errorf("spec cache: marshal: %w", err)
	}
	return os.WriteFile(cacheFile(cacheDir, apiName), data, 0o644)
}

// LoadFromCache reads the cached spec for apiName, re-parses it using loaders,
// and returns the result. Returns nil, nil when the cache is empty or expired.
func LoadFromCache(cacheDir, apiName, version string, loaders []Loader) (*APISpec, error) {
	entry, ok := readCache(cacheDir, apiName, version)
	if !ok {
		return nil, nil
	}
	return load(entry.ContentType, entry.Raw, loaders)
}

// InvalidateCache removes the cached spec file for apiName.
// Returns nil if the file did not exist.
func InvalidateCache(cacheDir, apiName string) error {
	err := os.Remove(cacheFile(cacheDir, apiName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
