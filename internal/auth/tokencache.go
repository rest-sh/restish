package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	configpkg "github.com/danielgtaylor/restish/v2/internal/config"
)

// CachedToken holds a cached OAuth2 access token and optional refresh token.
type CachedToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

// IsExpired reports whether the token is expired (or will expire within 30s).
func (t *CachedToken) IsExpired() bool {
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().Add(30 * time.Second).After(t.Expiry)
}

// TokenCache persists OAuth2 tokens as a flat JSON map at a given file path.
// All operations are safe for concurrent use.
type TokenCache struct {
	path    string
	mu      sync.Mutex
	loaded  bool
	cache   map[string]CachedToken
	modTime time.Time
}

// NewTokenCache returns a TokenCache that stores tokens at path.
func NewTokenCache(path string) *TokenCache {
	return &TokenCache{path: path}
}

// Get returns the cached token for key, or (nil, nil) if not found.
func (c *TokenCache) Get(key string) (*CachedToken, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, err := c.load()
	if err != nil {
		return nil, err
	}
	t, ok := m[key]
	if !ok {
		return nil, nil
	}
	return &t, nil
}

// Set stores token under key, creating or updating the cache file.
func (c *TokenCache) Set(key string, token CachedToken) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	lock, err := configpkg.LockSiblingFile(c.path)
	if err != nil {
		return err
	}
	defer lock.Close()
	m, err := c.loadLocked()
	if err != nil {
		return err
	}
	m[key] = token
	return c.saveLocked(m)
}

// Delete removes the entry for key. Returns nil when key is absent.
func (c *TokenCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	lock, err := configpkg.LockSiblingFile(c.path)
	if err != nil {
		return err
	}
	defer lock.Close()
	m, err := c.loadLocked()
	if err != nil {
		return err
	}
	delete(m, key)
	return c.saveLocked(m)
}

// DeletePrefix removes every cached token whose key begins with prefix.
func (c *TokenCache) DeletePrefix(prefix string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	lock, err := configpkg.LockSiblingFile(c.path)
	if err != nil {
		return err
	}
	defer lock.Close()
	m, err := c.loadLocked()
	if err != nil {
		return err
	}
	for key := range m {
		if strings.HasPrefix(key, prefix) {
			delete(m, key)
		}
	}
	return c.saveLocked(m)
}

func (c *TokenCache) load() (map[string]CachedToken, error) {
	if c.loaded {
		info, err := os.Stat(c.path)
		if err == nil && info.ModTime().Equal(c.modTime) {
			return c.cache, nil
		}
		if errors.Is(err, os.ErrNotExist) && c.modTime.IsZero() {
			return c.cache, nil
		}
	}
	return c.loadLocked()
}

func (c *TokenCache) loadLocked() (map[string]CachedToken, error) {
	data, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		c.cache = map[string]CachedToken{}
		c.loaded = true
		c.modTime = time.Time{}
		return c.cache, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]CachedToken
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	info, statErr := os.Stat(c.path)
	if statErr == nil {
		c.modTime = info.ModTime()
	}
	c.cache = m
	c.loaded = true
	return c.cache, nil
}

func (c *TokenCache) saveLocked(m map[string]CachedToken) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Atomic write: create a 0600 temp file in the target directory, then rename.
	tmp, err := secureCreateTemp(dir, "tokens-", ".tmp", 0600)
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmpName)
		if werr != nil {
			return werr
		}
		return cerr
	}
	if err := os.Rename(tmpName, c.path); err != nil {
		return err
	}
	info, statErr := os.Stat(c.path)
	if statErr == nil {
		c.modTime = info.ModTime()
	}
	c.cache = m
	c.loaded = true
	return nil
}

func secureCreateTemp(dir, prefix, suffix string, mode os.FileMode) (*os.File, error) {
	for range 100 {
		var buf [8]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return nil, err
		}
		name := filepath.Join(dir, fmt.Sprintf("%s%x%s", prefix, buf[:], suffix))
		f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, mode)
		if err == nil {
			return f, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("creating secure temp file in %s: too many collisions", dir)
}
