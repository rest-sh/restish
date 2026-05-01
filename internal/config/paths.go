package config

import (
	"os"
	"path/filepath"
	"runtime"
)

var userConfigDirFunc = os.UserConfigDir
var userCacheDirFunc = os.UserCacheDir
var userHomeDirFunc = os.UserHomeDir
var runtimeGOOS = runtime.GOOS

// Paths provides centralized directory and path management for Restish,
// using developer-friendly XDG-style defaults on Unix-like systems and
// standard user directories on Windows after Restish-specific environment
// variable overrides.
type Paths struct {
	// ConfigDir overrides (in order of precedence):
	// 1. RSH_CONFIG_DIR env var
	// 2. XDG_CONFIG_HOME/restish
	// 3. ~/.config/restish on Unix-like systems, os.UserConfigDir()/restish on Windows
	configDir string

	// CacheDir overrides (in order of precedence):
	// 1. RSH_CACHE_DIR env var
	// 2. XDG_CACHE_HOME/restish
	// 3. ~/.cache/restish on Unix-like systems, os.UserCacheDir()/restish on Windows
	cacheDir string
	// ConfigFile is the explicit config file path when RSH_CONFIG or
	// --rsh-config selects a file instead of the platform default directory.
	configFile string
}

// NewPaths creates a new Paths instance that computes directories
// based on the current environment and OS.
func NewPaths() *Paths {
	if file := os.Getenv("RSH_CONFIG"); file != "" {
		return NewPathsWithConfigFile(file)
	}
	return &Paths{
		configDir: computeConfigDir(),
		cacheDir:  computeCacheDir(),
	}
}

// NewPathsWithConfigFile creates Paths for an explicit config file. Sidecar
// state stored in the config directory, such as token caches and external-tool
// approvals, follows the selected config file.
func NewPathsWithConfigFile(path string) *Paths {
	return &Paths{
		configDir:  filepath.Dir(path),
		configFile: path,
		cacheDir:   computeCacheDir(),
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
	if p.configFile != "" {
		return p.configFile
	}
	return filepath.Join(p.configDir, "restish.json")
}

// computeConfigDir determines the configuration directory, respecting Restish's
// explicit override before using the default config directory.
func computeConfigDir() string {
	if dir := os.Getenv("RSH_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" && filepath.IsAbs(dir) {
		return filepath.Join(dir, "restish")
	}
	if runtimeGOOS == "windows" {
		if dir, err := userConfigDirFunc(); err == nil && dir != "" {
			return filepath.Join(dir, "restish")
		}
	} else if home, err := userHomeDirFunc(); err == nil && home != "" {
		return filepath.Join(home, ".config", "restish")
	}
	return ".restish"
}

// computeCacheDir determines the cache directory, respecting Restish's explicit
// override before using the default cache directory.
func computeCacheDir() string {
	if dir := os.Getenv("RSH_CACHE_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" && filepath.IsAbs(dir) {
		return filepath.Join(dir, "restish")
	}
	if runtimeGOOS == "windows" {
		if dir, err := userCacheDirFunc(); err == nil && dir != "" {
			return filepath.Join(dir, "restish")
		}
	} else if home, err := userHomeDirFunc(); err == nil && home != "" {
		return filepath.Join(home, ".cache", "restish")
	}
	return filepath.Join(".restish", "cache")
}
