package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// writeScript writes an executable shell script (or .bat on Windows) to dir
// and returns its full path. On Unix the script is made executable via chmod.
func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".bat"
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatalf("writeScript %s: %v", name, err)
	}
	return p
}

// jsonManifest returns a JSON-encoded Manifest for a plugin.
func jsonManifest(m Manifest) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// ---- isExecutable ---------------------------------------------------------

func TestIsExecutable_Executable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping chmod-based test on Windows")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "restish-test")
	os.WriteFile(p, []byte("#!/bin/sh\necho hi"), 0o755)
	if !isExecutable(p) {
		t.Error("expected executable")
	}
}

func TestIsExecutable_NotExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping chmod-based test on Windows")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "not-exec")
	os.WriteFile(p, []byte("data"), 0o644)
	if isExecutable(p) {
		t.Error("expected non-executable")
	}
}

func TestIsExecutable_Directory(t *testing.T) {
	dir := t.TempDir()
	if isExecutable(dir) {
		t.Error("directory should not be reported as executable")
	}
}

func TestIsExecutable_Nonexistent(t *testing.T) {
	if isExecutable("/nonexistent/restish-plugin") {
		t.Error("nonexistent file should not be executable")
	}
}

// ---- DefaultPluginDir -----------------------------------------------------

func TestDefaultPluginDir_RSHConfigDir(t *testing.T) {
	t.Setenv("RSH_CONFIG_DIR", "/custom/config")
	dir := DefaultPluginDir()
	if dir != filepath.Join("/custom/config", "plugins") {
		t.Errorf("got %q, want %q", dir, "/custom/config/plugins")
	}
}

func TestDefaultPluginDir_HomeDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping Unix home-dir test on Windows")
	}
	t.Setenv("RSH_CONFIG_DIR", "")
	dir := DefaultPluginDir()
	if dir == "" {
		t.Error("expected non-empty plugin dir")
	}
}

// ---- LoadManifest ---------------------------------------------------------

func TestLoadManifest_JSONOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	m := Manifest{
		Name:              "myplugin",
		Version:           "1.0.0",
		Description:       "A test plugin",
		RestishAPIVersion: CurrentPluginAPIVersion,
		Hooks:             []string{"auth"},
	}
	script := fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m))
	p := writeScript(t, dir, "restish-myplugin", script)

	got, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got.Name != "myplugin" {
		t.Errorf("Name: got %q, want %q", got.Name, "myplugin")
	}
	if got.RestishAPIVersion != CurrentPluginAPIVersion {
		t.Errorf("RestishAPIVersion: got %d, want %d", got.RestishAPIVersion, CurrentPluginAPIVersion)
	}
}

func TestLoadManifest_MissingName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	m := Manifest{RestishAPIVersion: 1} // no name
	script := fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m))
	p := writeScript(t, dir, "restish-noname", script)

	_, err := LoadManifest(p)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestLoadManifest_MissingAPIVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	m := Manifest{Name: "test"} // no api version
	script := fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m))
	p := writeScript(t, dir, "restish-nover", script)

	_, err := LoadManifest(p)
	if err == nil {
		t.Error("expected error for missing restish_api_version")
	}
}

func TestLoadManifest_EmptyOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	p := writeScript(t, dir, "restish-empty", "#!/bin/sh\n# no output")

	_, err := LoadManifest(p)
	if err == nil {
		t.Error("expected error for empty manifest output")
	}
}

func TestLoadManifest_NonZeroExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	p := writeScript(t, dir, "restish-fail", "#!/bin/sh\nexit 1")

	_, err := LoadManifest(p)
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
}

func TestLoadManifest_FutureVersion_WarnsButLoads(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	m := Manifest{
		Name:              "future",
		RestishAPIVersion: CurrentPluginAPIVersion + 1,
	}
	script := fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m))
	p := writeScript(t, dir, "restish-future", script)

	// Redirect warnings so they don't clutter test output.
	var warnBuf bytes.Buffer
	old := errorWriter
	errorWriter = &warnBuf
	defer func() { errorWriter = old }()

	got, err := LoadManifest(p)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil manifest for future-version plugin")
	}
	if warnBuf.Len() == 0 {
		t.Error("expected a warning to be emitted")
	}
}

// ---- Discover -------------------------------------------------------------

func TestDiscover_FindsPluginsInDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	m := Manifest{
		Name:              "myplugin",
		RestishAPIVersion: CurrentPluginAPIVersion,
	}
	script := fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m))
	writeScript(t, dir, "restish-myplugin", script)

	// Point PATH to our temp dir only so Discover finds our plugin.
	t.Setenv("PATH", dir)
	plugins := Discover("", nil, nil)
	if len(plugins) == 0 {
		t.Fatal("expected at least one plugin to be discovered")
	}
	if plugins[0].Manifest.Name != "myplugin" {
		t.Errorf("Name: got %q, want %q", plugins[0].Manifest.Name, "myplugin")
	}
}

func TestDiscover_SkipsBrokenPlugins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	// Broken plugin: exits non-zero.
	writeScript(t, dir, "restish-broken", "#!/bin/sh\nexit 1")

	t.Setenv("PATH", dir)
	var errs []string
	plugins := Discover("", nil, func(p string, err error) {
		errs = append(errs, err.Error())
	})
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(plugins))
	}
	if len(errs) == 0 {
		t.Error("expected errFn to be called for broken plugin")
	}
}

func TestDiscover_DeduplicatesPlugins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	m := Manifest{Name: "myplugin", RestishAPIVersion: CurrentPluginAPIVersion}
	script := fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m))
	writeScript(t, dir, "restish-myplugin", script)

	// Add dir twice to PATH to test deduplication.
	t.Setenv("PATH", dir+":"+dir)
	plugins := Discover("", nil, nil)
	if len(plugins) != 1 {
		t.Errorf("expected 1 unique plugin, got %d", len(plugins))
	}
}

func TestDiscover_EmptyPath_NoPlugins(t *testing.T) {
	t.Setenv("PATH", "")
	plugins := Discover("", nil, nil)
	if len(plugins) != 0 {
		t.Errorf("expected 0 plugins for empty PATH, got %d", len(plugins))
	}
}

func TestDiscover_AllowedPlugins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	dir := t.TempDir()
	m1 := Manifest{Name: "allowed", RestishAPIVersion: CurrentPluginAPIVersion}
	m2 := Manifest{Name: "blocked", RestishAPIVersion: CurrentPluginAPIVersion}
	writeScript(t, dir, "restish-allowed", fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m1)))
	writeScript(t, dir, "restish-blocked", fmt.Sprintf("#!/bin/sh\necho '%s'", jsonManifest(m2)))

	t.Setenv("PATH", dir)
	plugins := Discover("", []string{"restish-allowed"}, nil)
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Manifest.Name != "allowed" {
		t.Errorf("Name: got %q, want %q", plugins[0].Manifest.Name, "allowed")
	}
}
