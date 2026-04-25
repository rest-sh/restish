package config

import (
	"os"
	"path/filepath"
)

var userConfigDirFunc = os.UserConfigDir
var userCacheDirFunc = os.UserCacheDir

// Paths provides centralized directory and path management for Restish,
// relying on the standard library's platform paths after Restish-specific
// environment variable overrides.
type Paths struct {
	// ConfigDir overrides (in order of precedence):
	// 1. RSH_CONFIG_DIR env var
	// 2. os.UserConfigDir()/restish
	configDir string

	// CacheDir overrides (in order of precedence):
	// 1. RSH_CACHE_DIR env var
	// 2. os.UserCacheDir()/restish
	cacheDir string
}

// NewPaths creates a new Paths instance that computes directories
// based on the current environment and OS.
func NewPaths() *Paths {
	return &Paths{
		configDir: computeConfigDir(),
		cacheDir:  computeCacheDir(),
	}
}

// Config returns the config directory path, typically containing restish.json,
// profiles, and auth configuration.
func (p *Paths) Config() string {
	return p.configDir
}

// Cache returns the cache directory path for HTTP responses and other
// transient data.
func (p *Paths) Cache() string {
	return p.cacheDir
}

// SpecCache returns the subdirectory for cached API spec files.
func (p *Paths) SpecCache() string {
	return filepath.Join(p.cacheDir, "specs")
}

// TokenCache returns the path to the token cache file.
func (p *Paths) TokenCache() string {
	return filepath.Join(p.configDir, "tokens.cbor")
}

// PluginManifestCache returns the directory for cached plugin manifests.
func (p *Paths) PluginManifestCache() string {
	return filepath.Join(p.configDir, "plugin-manifest-cache.cbor")
}

// ConfigFile returns the path to the main restish.json config file.
func (p *Paths) ConfigFile() string {
	return filepath.Join(p.configDir, "restish.json")
}

// computeConfigDir determines the configuration directory, respecting Restish's
// explicit override before using the standard platform config directory.
func computeConfigDir() string {
	if dir := os.Getenv("RSH_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir, err := userConfigDirFunc(); err == nil && dir != "" {
		return filepath.Join(dir, "restish")
	}
	return ".restish"
}

// computeCacheDir determines the cache directory, respecting Restish's explicit
// override before using the standard platform cache directory.
func computeCacheDir() string {
	if dir := os.Getenv("RSH_CACHE_DIR"); dir != "" {
		return dir
	}
	if dir, err := userCacheDirFunc(); err == nil && dir != "" {
		return filepath.Join(dir, "restish")
	}
	return filepath.Join(".restish", "cache")
}
