// Package cache provides a size-bounded, LRU-evicting disk cache that
// implements the httpcache.Cache interface.  Responses are stored under a
// per-hostname subdirectory so they can be cleared by API name.
package cache

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	configpkg "github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/fileutil"
)

// DefaultMaxBytes is the default maximum cache size (100 MiB).
const DefaultMaxBytes = 100 * 1024 * 1024

const (
	cacheKeyLockShards      = 64
	cacheMtimeTouchInterval = 5 * time.Second
)

var renameCacheFile = os.Rename

// DiskCache is a file-system backed HTTP response cache.  It satisfies the
// httpcache.Cache interface (Get/Set/Delete) and additionally provides Info
// and Clear for the "restish cache" subcommands.
type DiskCache struct {
	dir          string
	namespace    string
	maxBytes     int64
	sizeEstimate int64      // atomic: running byte-count estimate; may drift upward
	evictMu      sync.Mutex // only one eviction goroutine at a time
	evictQueued  atomic.Bool
	evictWG      sync.WaitGroup
	keyLocks     [cacheKeyLockShards]sync.Mutex
	logger       io.Writer
}

// New returns a DiskCache rooted at dir with the given size cap.
// dir is created if it does not exist.
func New(dir string, maxBytes int64, namespace string) (*DiskCache, error) {
	return NewWithLogger(dir, maxBytes, namespace, nil)
}

// NewWithLogger returns a DiskCache that writes non-fatal cache diagnostics to
// logger when provided.
func NewWithLogger(dir string, maxBytes int64, namespace string, logger io.Writer) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("cache: create dir %s: %w", dir, err)
	}
	if namespace == "" {
		namespace = "_"
	}
	return &DiskCache{dir: dir, namespace: cachePathComponent(namespace), maxBytes: maxBytes, logger: logger}, nil
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
	return filepath.Join(c.dir, cachePathComponent(host), c.namespace, fmt.Sprintf("%x.cache", h))
}

func cachePathComponent(raw string) string {
	if raw == "" {
		return "_"
	}
	return url.QueryEscape(raw)
}

// Get returns the cached bytes for key, if present, and updates the file's
// mtime (used as LRU timestamp).
func (c *DiskCache) Get(key string) ([]byte, bool) {
	p := c.filePath(key)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	if info, err := os.Stat(p); err == nil && time.Since(info.ModTime()) >= cacheMtimeTouchInterval {
		now := time.Now()
		_ = os.Chtimes(p, now, now)
	}
	return data, true
}

// Set writes data to disk for key and evicts LRU entries if the total cache
// size exceeds the configured limit.
func (c *DiskCache) Set(key string, data []byte) {
	unlock := c.lockKey(key)
	defer unlock()

	p := c.filePath(key)
	var previousSize int64
	if info, err := os.Stat(p); err == nil {
		previousSize = info.Size()
	}
	if err := atomicWriteCacheFile(p, data); err != nil {
		if c.logger != nil {
			fmt.Fprintf(c.logger, "warning: cache write failed for %s: %v\n", p, err)
		}
		return
	}
	if previousSize > 0 {
		atomic.AddInt64(&c.sizeEstimate, -previousSize)
	}
	atomic.AddInt64(&c.sizeEstimate, int64(len(data)))
	c.evictIfNeeded()
}

func (c *DiskCache) lockKey(key string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	mu := &c.keyLocks[h.Sum32()%cacheKeyLockShards]
	mu.Lock()
	return mu.Unlock
}

func atomicWriteCacheFile(path string, data []byte) error {
	return fileutil.AtomicWriteFile(path, data, fileutil.AtomicWriteOptions{
		FileMode: 0o600,
		DirMode:  0o700,
		Rename:   renameCacheFile,
		SyncDir:  true,
	})
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
	// Keep at most one eviction worker queued/running. The worker re-checks the
	// estimate before exiting so writes that arrive mid-eviction are not missed.
	if !c.evictQueued.CompareAndSwap(false, true) {
		return
	}
	c.evictWG.Add(1)
	go func() {
		defer func() {
			c.evictQueued.Store(false)
			if atomic.LoadInt64(&c.sizeEstimate) > c.maxBytes {
				c.evictIfNeeded()
			}
			c.evictWG.Done()
		}()
		c.evictMu.Lock()
		defer c.evictMu.Unlock()
		lock, err := configpkg.LockSiblingFile(filepath.Join(c.dir, ".evict"))
		if err != nil {
			return
		}
		defer lock.Close()
		for {
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
		}
	}()
}

// WaitEvict blocks until any pending background eviction goroutine has
// finished. It is intended for use in tests only.
func (c *DiskCache) WaitEvict() {
	c.evictWG.Wait()
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

// Clear deletes HTTP response cache entries. If host is non-empty, only entries
// stored under the <host> subdirectory are removed (i.e. responses for one API).
func (c *DiskCache) Clear(host string) error {
	if host == "" {
		entries, _, err := c.allFiles()
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			_ = os.Remove(e.path)
		}
		atomic.StoreInt64(&c.sizeEstimate, 0)
		return nil
	}
	target := filepath.Join(c.dir, cachePathComponent(host))
	if _, err := os.Stat(target); err != nil {
		if errors.Is(err, os.ErrNotExist) && cachePathComponent(host) != host {
			legacy := filepath.Join(c.dir, host)
			if _, legacyErr := os.Stat(legacy); legacyErr == nil {
				err := os.RemoveAll(legacy)
				c.recomputeSizeEstimate()
				return err
			}
		}
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no cached entries for host %q", host)
		}
		return err
	}
	err := os.RemoveAll(target)
	c.recomputeSizeEstimate()
	return err
}

// ClearNamespaces deletes entries for the given cache namespaces across all
// hosts. It is used for API-specific clearing because one registered API can
// have multiple profile base URLs or operation hosts that share a hostname with
// other APIs.
func (c *DiskCache) ClearNamespaces(namespaces []string) error {
	if len(namespaces) == 0 {
		return nil
	}
	want := make(map[string]bool, len(namespaces))
	for _, namespace := range namespaces {
		if namespace != "" {
			want[namespace] = true
			want[cachePathComponent(namespace)] = true
		}
	}
	err := filepath.Walk(c.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if want[info.Name()] {
			_ = os.RemoveAll(path)
			return filepath.SkipDir
		}
		return nil
	})
	c.recomputeSizeEstimate()
	return err
}

// ClearNamespacePrefix deletes entries for every namespace with prefix.
func (c *DiskCache) ClearNamespacePrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	encodedPrefix := cachePathComponent(prefix)
	hosts, err := os.ReadDir(c.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, host := range hosts {
		if !host.IsDir() {
			continue
		}
		hostPath := filepath.Join(c.dir, host.Name())
		namespaces, err := os.ReadDir(hostPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		for _, namespace := range namespaces {
			if !namespace.IsDir() {
				continue
			}
			if stringsHasNamespacePrefix(namespace.Name(), prefix, encodedPrefix) {
				_ = os.RemoveAll(filepath.Join(hostPath, namespace.Name()))
			}
		}
	}
	c.recomputeSizeEstimate()
	return nil
}

func stringsHasNamespacePrefix(name, rawPrefix, encodedPrefix string) bool {
	return strings.HasPrefix(name, rawPrefix) || strings.HasPrefix(name, encodedPrefix)
}

func (c *DiskCache) recomputeSizeEstimate() {
	_, total, err := c.allFiles()
	if err != nil {
		total = 0
	}
	atomic.StoreInt64(&c.sizeEstimate, total)
	c.evictIfNeeded()
}
