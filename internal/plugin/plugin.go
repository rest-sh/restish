// Package plugin handles discovery, manifest loading, and caching of Restish
// out-of-process plugins. Plugins are executables named restish-<name> found
// on PATH or in the plugin directory.
package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
)

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
}

// Plugin is a discovered plugin executable together with its manifest.
type Plugin struct {
	Path     string
	Manifest Manifest
}

// Discover finds all restish-* plugins on PATH and in pluginDir.
// Errors loading individual plugin manifests are reported via errFn but do not
// abort discovery. Pass nil for errFn to silently skip broken plugins.
func Discover(pluginDir string, errFn func(path string, err error)) []Plugin {
	seen := map[string]bool{}
	var plugins []Plugin

	add := func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		m, err := LoadManifest(path)
		if err != nil {
			if errFn != nil {
				errFn(path, err)
			}
			return
		}
		plugins = append(plugins, Plugin{Path: path, Manifest: *m})
	}

	// Scan PATH.
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

	// Scan plugin dir.
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
