package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	path string
	mu   sync.Mutex
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
	m, err := c.load()
	if err != nil {
		return err
	}
	m[key] = token
	return c.save(m)
}

// Delete removes the entry for key. Returns nil when key is absent.
func (c *TokenCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, err := c.load()
	if err != nil {
		return err
	}
	delete(m, key)
	return c.save(m)
}

func (c *TokenCache) load() (map[string]CachedToken, error) {
	data, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]CachedToken{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m map[string]CachedToken
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *TokenCache) save(m map[string]CachedToken) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Atomic write: write to a temp file, set permissions, then rename.
	tmp, err := os.CreateTemp(dir, "tokens-*.tmp")
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
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, c.path)
}
