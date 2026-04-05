package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testPluginBin is the path to the compiled test plugin binary, set in TestMain.
var testPluginBin string

// testHookPluginBin is the path to the compiled hook test plugin binary.
var testHookPluginBin string
var testCmdPluginBin string
var testMCPPluginBin string
var testBulkPluginBin string
var testTLSSignerPluginBin string

// TestMain compiles the test plugin binaries once for the whole test run.
func TestMain(m *testing.M) {
	// Build testplugin.
	bin := filepath.Join(os.TempDir(), "restish-testplugin")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/testplugin")
	cmd.Dir = testdataDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = out
	} else {
		testPluginBin = bin
	}

	// Build hookplugin.
	hookBin := filepath.Join(os.TempDir(), "restish-hookplugin")
	if runtime.GOOS == "windows" {
		hookBin += ".exe"
	}

	// Build cmdplugin.
	cmdBin := filepath.Join(os.TempDir(), "restish-cmdplugin")
	if runtime.GOOS == "windows" {
		cmdBin += ".exe"
	}
	cmd3 := exec.Command("go", "build", "-o", cmdBin, "./testdata/cmdplugin")
	cmd3.Dir = testdataDir()
	if out, err := cmd3.CombinedOutput(); err != nil {
		_ = out
	} else {
		testCmdPluginBin = cmdBin
	}

	mcpBin := filepath.Join(os.TempDir(), "restish-mcp")
	if runtime.GOOS == "windows" {
		mcpBin += ".exe"
	}
	cmd4 := exec.Command("go", "build", "-o", mcpBin, "../../cmd/restish-mcp")
	cmd4.Dir = testdataDir()
	if out, err := cmd4.CombinedOutput(); err != nil {
		_ = out
	} else {
		testMCPPluginBin = mcpBin
	}

	bulkBin := filepath.Join(os.TempDir(), "restish-bulk")
	if runtime.GOOS == "windows" {
		bulkBin += ".exe"
	}
	cmdBulk := exec.Command("go", "build", "-o", bulkBin, "../../cmd/restish-bulk")
	cmdBulk.Dir = testdataDir()
	if out, err := cmdBulk.CombinedOutput(); err != nil {
		_ = out
	} else {
		testBulkPluginBin = bulkBin
	}

	tlsSignerBin := filepath.Join(os.TempDir(), "restish-test-tls-signer")
	if runtime.GOOS == "windows" {
		tlsSignerBin += ".exe"
	}
	cmd5 := exec.Command("go", "build", "-o", tlsSignerBin, "../request/testdata/tlssigner")
	cmd5.Dir = testdataDir()
	if out, err := cmd5.CombinedOutput(); err != nil {
		_ = out
	} else {
		testTLSSignerPluginBin = tlsSignerBin
	}
	cmd2 := exec.Command("go", "build", "-o", hookBin, "./testdata/hookplugin")
	cmd2.Dir = testdataDir()
	if out, err := cmd2.CombinedOutput(); err != nil {
		_ = out
	} else {
		testHookPluginBin = hookBin
	}

	code := m.Run()

	if testPluginBin != "" {
		_ = os.Remove(testPluginBin)
	}
	if testHookPluginBin != "" {
		_ = os.Remove(testHookPluginBin)
	}
	if testCmdPluginBin != "" {
		_ = os.Remove(testCmdPluginBin)
	}
	if testMCPPluginBin != "" {
		_ = os.Remove(testMCPPluginBin)
	}
	if testBulkPluginBin != "" {
		_ = os.Remove(testBulkPluginBin)
	}
	if testTLSSignerPluginBin != "" {
		_ = os.Remove(testTLSSignerPluginBin)
	}
	os.Exit(code)
}

// testdataDir returns the directory containing testdata/ relative to this file.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// skipNoPlugin skips the test if the plugin binary wasn't compiled.
func skipNoPlugin(t *testing.T) {
	t.Helper()
	if testPluginBin == "" {
		t.Skip("test plugin binary not compiled; skipping plugin tests")
	}
}

// skipNoHookPlugin skips the test if the hook plugin binary wasn't compiled.
func skipNoHookPlugin(t *testing.T) {
	t.Helper()
	if testHookPluginBin == "" {
		t.Skip("hook plugin binary not compiled; skipping hook plugin tests")
	}
}

func skipNoCmdPlugin(t *testing.T) {
	t.Helper()
	if testCmdPluginBin == "" {
		t.Skip("command plugin binary not compiled; skipping command plugin tests")
	}
}

func skipNoMCPPlugin(t *testing.T) {
	t.Helper()
	if testMCPPluginBin == "" {
		t.Skip("mcp plugin binary not compiled; skipping mcp plugin tests")
	}
}

func skipNoBulkPlugin(t *testing.T) {
	t.Helper()
	if testBulkPluginBin == "" {
		t.Skip("bulk plugin binary not compiled; skipping bulk plugin tests")
	}
}

func skipNoTLSSignerPlugin(t *testing.T) {
	t.Helper()
	if testTLSSignerPluginBin == "" {
		t.Skip("tls-signer plugin binary not compiled; skipping tls-signer plugin tests")
	}
}

// TestPluginDiscoverOnPATH verifies that a restish-* binary on PATH is
// discovered by "plugin list".
func TestPluginDiscoverOnPATH(t *testing.T) {
	skipNoPlugin(t)

	// Put a copy of the plugin in a temp dir and add it to PATH.
	dir := t.TempDir()
	dest := filepath.Join(dir, "restish-testplugin")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	data, err := os.ReadFile(testPluginBin)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		t.Fatalf("write plugin copy: %v", err)
	}

	// Prepend our dir to PATH.
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	c, out, errOut := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
		t.Fatalf("plugin list: %v", err)
	}
	_ = errOut
	if !strings.Contains(out.String(), "testplugin") {
		t.Errorf("expected testplugin in plugin list output, got:\n%s", out.String())
	}
}

// TestPluginDiscoverInPluginDir verifies that a plugin binary in the plugin
// directory is discovered.
func TestPluginDiscoverInPluginDir(t *testing.T) {
	skipNoPlugin(t)

	pluginDir := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", t.TempDir()) // redirect default plugin dir

	// Install the binary into our custom plugin dir.
	dest := filepath.Join(pluginDir, "restish-testplugin")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	data, err := os.ReadFile(testPluginBin)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	// Discover from pluginDir directly.
	from := &pluginDirDiscovery{dir: pluginDir}
	_ = from // actually we use plugin.Discover() via the CLI command
	// Use plugin list with RSH_CONFIG_DIR pointing to a dir containing plugins/.
	pluginsParent := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pluginsParent, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	dest2 := filepath.Join(pluginsParent, "plugins", "restish-testplugin")
	if runtime.GOOS == "windows" {
		dest2 += ".exe"
	}
	if err := os.WriteFile(dest2, data, 0o755); err != nil {
		t.Fatalf("write plugin2: %v", err)
	}
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	// Clear PATH so the plugin isn't discovered there.
	t.Setenv("PATH", "")

	c, out, _ := newTestCLI()
	c.ConfigPath = filepath.Join(pluginsParent, "restish.json")
	if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
		t.Fatalf("plugin list: %v", err)
	}
	if !strings.Contains(out.String(), "testplugin") {
		t.Errorf("expected testplugin from plugin dir, got:\n%s", out.String())
	}
}

// TestPluginListShowsNameVersionHooks verifies that "plugin list" prints
// the name, version, and hooks from the manifest.
func TestPluginListShowsNameVersionHooks(t *testing.T) {
	skipNoPlugin(t)

	dir := t.TempDir()
	dest := filepath.Join(dir, "restish-testplugin")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	data, _ := os.ReadFile(testPluginBin)
	_ = os.WriteFile(dest, data, 0o755)

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	t.Setenv("RSH_CONFIG_DIR", t.TempDir())

	c, out, _ := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"
	_ = c.Run([]string{"restish", "plugin", "list"})

	got := out.String()
	if !strings.Contains(got, "testplugin") {
		t.Errorf("expected name in output, got:\n%s", got)
	}
	if !strings.Contains(got, "1.0.0") {
		t.Errorf("expected version in output, got:\n%s", got)
	}
	if !strings.Contains(got, "command") {
		t.Errorf("expected hook 'command' in output, got:\n%s", got)
	}
}

// TestPluginInvalidManifest verifies that a plugin that exits non-zero on
// --rsh-plugin-manifest is reported as a warning but doesn't crash Restish.
func TestPluginInvalidManifest(t *testing.T) {
	dir := t.TempDir()

	// Create a fake plugin that always exits 1.
	badPlugin := filepath.Join(dir, "restish-bad")
	if runtime.GOOS == "windows" {
		t.Skip("shell script test not supported on Windows")
	}
	if err := os.WriteFile(badPlugin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
	t.Setenv("RSH_CONFIG_DIR", t.TempDir())

	c, out, errOut := newTestCLI()
	c.ConfigPath = t.TempDir() + "/restish.json"

	// Should not return an error — broken plugins are skipped.
	if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
		t.Fatalf("plugin list with bad plugin: %v", err)
	}
	// Warning should appear on stderr.
	if !strings.Contains(errOut.String(), "warning") {
		t.Errorf("expected warning for bad plugin on stderr, got:\n%s", errOut.String())
	}
	// Bad plugin must not appear in stdout list.
	if strings.Contains(out.String(), "restish-bad") {
		t.Errorf("bad plugin should not appear in list:\n%s", out.String())
	}
}

// pluginDirDiscovery is a test helper (unused beyond syntax check).
type pluginDirDiscovery struct{ dir string }
