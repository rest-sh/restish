// Package plugin handles discovery, manifest loading, and caching of Restish
// out-of-process plugins. Plugins are executables named restish-<name> found
// in the plugin directory.
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
	configpkg "github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/procutil"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

// CurrentPluginAPIVersion is the highest plugin protocol version this build of
// Restish understands. Plugins that declare a higher version may use protocol
// features that this host cannot handle; a warning is emitted during discovery.
const CurrentPluginAPIVersion = 2

// Manifest is an alias for the canonical plugin.Manifest defined in the public
// plugin package. Using the same type eliminates the dual-maintenance risk that
// arose when the two structs diverged.
type Manifest = pluginwire.Manifest

// Plugin is a discovered plugin executable together with its manifest.
type Plugin struct {
	Path     string
	Manifest Manifest
}

// Discover finds all executable restish-* plugins in pluginDir.
// Errors loading individual plugin manifests are reported via errFn but do not
// abort discovery. Pass nil for errFn to silently skip broken plugins.
// manifestCacheFile, when non-empty, enables a CBOR on-disk manifest cache
// keyed by plugin path + mtime. This avoids subprocess spawns on every
// invocation when the plugin binary has not changed. When duplicate plugin
// identities are found, the first plugin in directory order is loaded.
func Discover(pluginDir string, errFn func(path string, err error), manifestCacheFile string, stderr io.Writer) []Plugin {
	seenPaths := map[string]bool{}
	seenNames := map[string]bool{}
	var plugins []Plugin

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
			m, err := loadManifest(path, stderr)
			if err != nil {
				return nil, err
			}
			cache[path] = manifestCacheEntry{Mtime: mtime, Manifest: *m}
			cacheUpdated = true
			return m, nil
		}
		return loadManifest(path, stderr)
	}

	add := func(path string) {
		if seenPaths[path] {
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

	if pluginDir != "" {
		entries, err := os.ReadDir(pluginDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				full := filepath.Join(pluginDir, e.Name())
				if isPluginExecutableName(e.Name()) && isExecutable(full) {
					add(full)
				}
			}
		}
	}

	if cacheUpdated && manifestCacheFile != "" {
		saveManifestCache(manifestCacheFile, cache, stderr)
	}

	return plugins
}

// LoadManifest calls path with plugin.StartupFlagManifest and parses the CBOR (or
// JSON fallback) manifest from stdout.
func LoadManifest(path string) (*Manifest, error) {
	return loadManifest(path, os.Stderr)
}

// LoadManifestWithWarnings is like LoadManifest, but sends compatibility
// warnings to warningWriter. Pass nil to suppress warnings.
func LoadManifestWithWarnings(path string, warningWriter io.Writer) (*Manifest, error) {
	return loadManifest(path, warningWriter)
}

func loadManifest(path string, warningWriter io.Writer) (*Manifest, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, pluginwire.StartupFlagManifest)
	procutil.ConfigureCommandTreeKill(ctx, cmd)
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
		if warningWriter != nil {
			fmt.Fprintf(warningWriter, "warning: plugin %s declares restish_api_version %d but this host only supports %d; some features may not work\n",
				filepath.Base(path), m.RestishAPIVersion, CurrentPluginAPIVersion)
		}
	}

	return &m, nil
}

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

// DefaultPluginDir returns the default directory for installed plugins.
func DefaultPluginDir() string {
	return filepath.Join(configpkg.NewPaths().Config(), "plugins")
}

// DefaultManifestCachePath returns the path to the on-disk plugin manifest
// cache file. Stored next to the config file to be automatically cleaned up
// when users wipe their config directory.
func DefaultManifestCachePath() string {
	return configpkg.NewPaths().PluginManifestCache()
}

func isPluginExecutableName(name string) bool {
	return strings.HasPrefix(strings.TrimSuffix(name, ".exe"), "restish-")
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
