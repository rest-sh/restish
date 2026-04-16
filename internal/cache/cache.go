// Package cache provides a size-bounded, LRU-evicting disk cache that
// implements the httpcache.Cache interface.  Responses are stored under a
// per-hostname subdirectory so they can be cleared by API name.
package cache

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMaxBytes is the default maximum cache size (100 MiB).
const DefaultMaxBytes = 100 * 1024 * 1024

// DiskCache is a file-system backed HTTP response cache.  It satisfies the
// httpcache.Cache interface (Get/Set/Delete) and additionally provides Info
// and Clear for the "restish cache" subcommands.
type DiskCache struct {
	dir          string
	maxBytes     int64
	sizeEstimate int64      // atomic: running byte-count estimate; may drift upward
	evictMu      sync.Mutex // only one eviction goroutine at a time
}

// New returns a DiskCache rooted at dir with the given size cap.
// dir is created if it does not exist.
func New(dir string, maxBytes int64) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("cache: create dir %s: %w", dir, err)
	}
	return &DiskCache{dir: dir, maxBytes: maxBytes}, nil
}

// filePath derives the cache file path for a given URL key.
// Files are grouped under <dir>/<hostname>/<sha256(key)>.cache so that
// Clear("<hostname>") removes all entries for one API.
func (c *DiskCache) filePath(key string) string {
	host := "_"
	if u, err := url.Parse(key); err == nil && u.Host != "" {
		host = u.Host
	}
	h := sha256.Sum256([]byte(key))
	return filepath.Join(c.dir, host, fmt.Sprintf("%x.cache", h))
}

// Get returns the cached bytes for key, if present, and updates the file's
// mtime (used as LRU timestamp).
func (c *DiskCache) Get(key string) ([]byte, bool) {
	p := c.filePath(key)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	now := time.Now()
	_ = os.Chtimes(p, now, now)
	return data, true
}

// Set writes data to disk for key and evicts LRU entries if the total cache
// size exceeds the configured limit.
func (c *DiskCache) Set(key string, data []byte) {
	p := c.filePath(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(p, data, 0o600)
	atomic.AddInt64(&c.sizeEstimate, int64(len(data)))
	c.evictIfNeeded()
}

// Delete removes the cached entry for key.
func (c *DiskCache) Delete(key string) {
	_ = os.Remove(c.filePath(key))
}

// fileEntry holds metadata for one cache file used during LRU eviction.
type fileEntry struct {
	path  string
	size  int64
	mtime time.Time
}

// allFiles walks c.dir and returns all *.cache files with their sizes and
// mtimes, plus the grand total byte count.
func (c *DiskCache) allFiles() ([]fileEntry, int64, error) {
	var entries []fileEntry
	var total int64
	err := filepath.Walk(c.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".cache" {
			return nil
		}
		entries = append(entries, fileEntry{path: path, size: info.Size(), mtime: info.ModTime()})
		total += info.Size()
		return nil
	})
	return entries, total, err
}

// evictIfNeeded triggers a background LRU eviction pass when the in-memory
// size estimate exceeds maxBytes.  The estimate is conservative: it can drift
// upward (e.g. a key that was already present gets overwritten), so the real
// check is performed by the goroutine with an accurate directory walk.
func (c *DiskCache) evictIfNeeded() {
	if c.maxBytes <= 0 {
		return
	}
	// Fast-path: estimate is still under the cap; skip the directory walk.
	if atomic.LoadInt64(&c.sizeEstimate) <= c.maxBytes {
		return
	}
	// Only one eviction goroutine at a time; others can skip.
	if !c.evictMu.TryLock() {
		return
	}
	go func() {
		defer c.evictMu.Unlock()
		entries, total, err := c.allFiles()
		if err != nil {
			return
		}
		// Replace the estimate with the accurate value.
		atomic.StoreInt64(&c.sizeEstimate, total)
		if total <= c.maxBytes {
			return
		}
		// Sort oldest-first (LRU).
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].mtime.Before(entries[j].mtime)
		})
		for _, e := range entries {
			if total <= c.maxBytes {
				break
			}
			if err := os.Remove(e.path); err == nil {
				total -= e.size
			}
		}
		atomic.StoreInt64(&c.sizeEstimate, total)
	}()
}

// WaitEvict blocks until any pending background eviction goroutine has
// finished. It is intended for use in tests only.
func (c *DiskCache) WaitEvict() {
	c.evictMu.Lock()
	c.evictMu.Unlock() //nolint:staticcheck
}

// Info holds statistics about the cache.
type Info struct {
	SizeBytes   int64
	EntryCount  int
	OldestEntry time.Time
}

// Info returns current cache statistics.
func (c *DiskCache) Info() (*Info, error) {
	entries, total, err := c.allFiles()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	info := &Info{SizeBytes: total, EntryCount: len(entries)}
	for _, e := range entries {
		if info.OldestEntry.IsZero() || e.mtime.Before(info.OldestEntry) {
			info.OldestEntry = e.mtime
		}
	}
	return info, nil
}

// Clear deletes all cache entries.  If host is non-empty, only entries stored
// under the <host> subdirectory are removed (i.e. responses for one API).
func (c *DiskCache) Clear(host string) error {
	if host == "" {
		entries, err := os.ReadDir(c.dir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(c.dir, e.Name()))
		}
		return nil
	}
	return os.RemoveAll(filepath.Join(c.dir, host))
}
