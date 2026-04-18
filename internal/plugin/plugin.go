// Package plugin handles discovery, manifest loading, and caching of Restish
// out-of-process plugins. Plugins are executables named restish-<name> found
// on PATH or in the plugin directory.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// CurrentPluginAPIVersion is the highest plugin protocol version this build of
// Restish understands. Plugins that declare a higher version may use protocol
// features that this host cannot handle; a warning is emitted during discovery.
const CurrentPluginAPIVersion = 2

// errorWriter receives plugin-discovery warnings. Defaults to os.Stderr;
// tests may redirect it to suppress or capture output.
var errorWriter io.Writer = os.Stderr

// Manifest is the metadata a plugin reports when called with
// --rsh-plugin-manifest.
type Manifest struct {
	Name              string   `json:"name"                cbor:"name"`
	Version           string   `json:"version"             cbor:"version"`
	Description       string   `json:"description"         cbor:"description"`
	RestishAPIVersion int      `json:"restish_api_version" cbor:"restish_api_version"`
	Hooks             []string `json:"hooks,omitempty"     cbor:"hooks,omitempty"`
	// FormatterNames lists the output format names this plugin handles when
	// the "formatter" hook is declared. Each name is available via -o <name>.
	FormatterNames []string `json:"formatter_names,omitempty" cbor:"formatter_names,omitempty"`
	// LoaderContentTypes lists the MIME types this plugin can convert to an
	// OpenAPI descriptor when the "loader" hook is declared.
	LoaderContentTypes []string `json:"loader_content_types,omitempty" cbor:"loader_content_types,omitempty"`
	// AuthAPINames, when non-empty, limits the "auth" hook to only fire for
	// requests targeting the named APIs. Plugins that handle auth for a
	// specific API should declare this to avoid unnecessary subprocess spawns
	// for all other APIs.
	AuthAPINames []string `json:"auth_api_names,omitempty" cbor:"auth_api_names,omitempty"`
	// NeedsAuthSecrets, when true, causes the full auth params map (including
	// secret values such as passwords and client secrets) to be forwarded to
	// this plugin's auth hook. When false (the default) secret params are
	// omitted before dispatch to reduce secret exposure.
	NeedsAuthSecrets bool `json:"needs_auth_secrets,omitempty" cbor:"needs_auth_secrets,omitempty"`
}

// Plugin is a discovered plugin executable together with its manifest.
type Plugin struct {
	Path     string
	Manifest Manifest
}

// Discover finds all restish-* plugins in pluginDir and on PATH.
// Errors loading individual plugin manifests are reported via errFn but do not
// abort discovery. Pass nil for errFn to silently skip broken plugins.
// When allowedPlugins is non-empty only executables whose base name appears in
// the list are loaded; all others are silently skipped.
// manifestCacheFile, when non-empty, enables a CBOR on-disk manifest cache
// keyed by plugin path + mtime. This avoids subprocess spawns on every
// invocation when the plugin binary has not changed. When duplicate plugin
// identities are found, pluginDir takes precedence over PATH.
func Discover(pluginDir string, allowedPlugins []string, errFn func(path string, err error), manifestCacheFile string) []Plugin {
	seenPaths := map[string]bool{}
	seenNames := map[string]bool{}
	var plugins []Plugin

	allowed := make(map[string]bool, len(allowedPlugins))
	for _, name := range allowedPlugins {
		allowed[name] = true
	}

	cache := loadManifestCache(manifestCacheFile)
	cacheUpdated := false

	resolveManifest := func(path string) (*Manifest, error) {
		info, statErr := os.Stat(path)
		if statErr == nil && manifestCacheFile != "" {
			mtime := info.ModTime().UnixNano()
			if entry, ok := cache[path]; ok && entry.Mtime == mtime {
				m := entry.Manifest
				return &m, nil
			}
			m, err := LoadManifest(path)
			if err != nil {
				return nil, err
			}
			cache[path] = manifestCacheEntry{Mtime: mtime, Manifest: *m}
			cacheUpdated = true
			return m, nil
		}
		return LoadManifest(path)
	}

	add := func(path string) {
		if seenPaths[path] {
			return
		}
		if len(allowed) > 0 && !allowed[filepath.Base(path)] {
			return
		}
		seenPaths[path] = true
		m, err := resolveManifest(path)
		if err != nil {
			if errFn != nil {
				errFn(path, err)
			}
			return
		}
		if seenNames[m.Name] {
			return
		}
		seenNames[m.Name] = true
		plugins = append(plugins, Plugin{Path: path, Manifest: *m})
	}

	// Scan plugin dir first so explicitly installed plugins override PATH
	// plugins with the same declared identity.
	if pluginDir != "" {
		entries, err := os.ReadDir(pluginDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				full := filepath.Join(pluginDir, e.Name())
				if isExecutable(full) {
					add(full)
				}
			}
		}
	}

	// Scan PATH.
	if len(allowed) > 0 {
		// Fast path: look up each allowed plugin directly instead of
		// enumerating every entry in every PATH directory.
		for name := range allowed {
			if full, err := exec.LookPath(name); err == nil {
				add(full)
			}
		}
	} else {
		for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if !strings.HasPrefix(name, "restish-") {
					continue
				}
				full := filepath.Join(dir, name)
				if isExecutable(full) {
					add(full)
				}
			}
		}
	}

	if cacheUpdated && manifestCacheFile != "" {
		saveManifestCache(manifestCacheFile, cache)
	}

	return plugins
}

// LoadManifest calls path with --rsh-plugin-manifest and parses the CBOR (or
// JSON fallback) manifest from stdout.
func LoadManifest(path string) (*Manifest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--rsh-plugin-manifest")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plugin %s: manifest exec: %w", filepath.Base(path), err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("plugin %s: empty manifest", filepath.Base(path))
	}

	var m Manifest
	// Try CBOR first, then JSON.
	if err := cbor.Unmarshal(out, &m); err != nil {
		if err2 := json.Unmarshal(out, &m); err2 != nil {
			return nil, fmt.Errorf("plugin %s: invalid manifest (cbor: %v; json: %v)", filepath.Base(path), err, err2)
		}
	}

	if m.Name == "" {
		return nil, fmt.Errorf("plugin %s: manifest missing name", filepath.Base(path))
	}
	if m.RestishAPIVersion < 1 {
		return nil, fmt.Errorf("plugin %s: manifest missing or invalid restish_api_version", filepath.Base(path))
	}
	if m.RestishAPIVersion > CurrentPluginAPIVersion {
		// Warn but still load: the plugin may work for the features it actually uses.
		fmt.Fprintf(errorWriter, "warning: plugin %s declares restish_api_version %d but this host only supports %d; some features may not work\n",
			filepath.Base(path), m.RestishAPIVersion, CurrentPluginAPIVersion)
	}

	return &m, nil
}

// DefaultPluginDir returns the default directory for installed plugins.
func DefaultPluginDir() string {
	if dir := os.Getenv("RSH_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "plugins")
	}
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "restish", "plugins")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "restish", "plugins")
	}
	return filepath.Join(".restish", "plugins")
}

// DefaultManifestCachePath returns the path to the on-disk plugin manifest
// cache file. Stored next to the config file to be automatically cleaned up
// when users wipe their config directory.
func DefaultManifestCachePath() string {
	if dir := os.Getenv("RSH_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "plugin-manifest-cache.cbor")
	}
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "restish", "plugin-manifest-cache.cbor")
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "restish", "plugin-manifest-cache.cbor")
	}
	return filepath.Join(".restish", "plugin-manifest-cache.cbor")
}

// isExecutable reports whether path is a regular executable file.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".exe" || ext == ".cmd" || ext == ".bat"
	}
	return info.Mode()&0o111 != 0
}
