package config

import (
	"os"
	"path/filepath"
	"runtime"
)

var userConfigDirFunc = os.UserConfigDir
var userCacheDirFunc = os.UserCacheDir
var runtimeGOOS = runtime.GOOS

// Paths provides centralized directory and path management for Restish,
// supporting XDG Base Directory specification on Unix-like systems and
// standard Windows directories. Paths honor explicit environment variable
// overrides for each category (RSH_CONFIG_DIR, RSH_CACHE_DIR, etc.).
type Paths struct {
	// ConfigDir overrides (in order of precedence):
	// 1. RSH_CONFIG_DIR env var
	// 2. XDG_CONFIG_HOME env var (Unix-like)
	// 3. %APPDATA% env var (Windows)
	// 4. ~/.config (Unix-like) or %UserProfile%\AppData\Roaming (Windows)
	configDir string

	// CacheDir overrides (in order of precedence):
	// 1. RSH_CACHE_DIR env var
	// 2. XDG_CACHE_HOME env var (Unix-like)
	// 3. %LOCALAPPDATA% env var (Windows)
	// 4. ~/.cache (Unix-like) or %UserProfile%\AppData\Local (Windows)
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

// computeConfigDir determines the configuration directory, respecting
// environment variable overrides and platform conventions.
func computeConfigDir() string {
	// Explicit override
	if dir := os.Getenv("RSH_CONFIG_DIR"); dir != "" {
		return dir
	}

	// Platform-specific defaults
	if runtimeGOOS == "windows" {
		// Windows: %APPDATA%\restish (or %UserProfile%\AppData\Roaming\restish)
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "restish")
		}
		if dir, err := userConfigDirFunc(); err == nil && dir != "" {
			return filepath.Join(dir, "restish")
		}
	} else {
		// Unix-like: XDG_CONFIG_HOME/restish or ~/.config/restish
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			return filepath.Join(xdgConfig, "restish")
		}
		if dir, err := userConfigDirFunc(); err == nil && dir != "" {
			return filepath.Join(dir, "restish")
		}
	}

	// Fallback
	return ".restish"
}

// computeCacheDir determines the cache directory, respecting environment
// variable overrides and platform conventions.
func computeCacheDir() string {
	// Explicit override
	if dir := os.Getenv("RSH_CACHE_DIR"); dir != "" {
		return dir
	}

	// Platform-specific defaults
	if runtimeGOOS == "windows" {
		// Windows: %LOCALAPPDATA%\restish\cache
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "restish", "cache")
		}
		if dir, err := userCacheDirFunc(); err == nil && dir != "" {
			return filepath.Join(dir, "restish")
		}
	} else {
		// Unix-like: XDG_CACHE_HOME/restish or ~/.cache/restish
		if xdgCache := os.Getenv("XDG_CACHE_HOME"); xdgCache != "" {
			return filepath.Join(xdgCache, "restish")
		}
		if dir, err := userCacheDirFunc(); err == nil && dir != "" {
			return filepath.Join(dir, "restish")
		}
	}

	// Fallback
	return filepath.Join(".restish", "cache")
}
