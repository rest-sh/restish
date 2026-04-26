package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var testPluginBuildDir string

// testPluginBin values are populated lazily by the skipNo* helpers below.
var (
	testPluginBin          string
	testHookPluginBin      string
	testCmdPluginBin       string
	testMCPPluginBin       string
	testBulkPluginBin      string
	testTLSSignerPluginBin string
	testCSVPluginBin       string
)

var (
	testPluginBuilder = testPluginBuild{
		name: "restish-testplugin",
		pkg:  "./testdata/testplugin",
		bin:  &testPluginBin,
	}
	testHookPluginBuilder = testPluginBuild{
		name:   "restish-hookplugin",
		source: &testPluginBuilder,
		bin:    &testHookPluginBin,
	}
	testCmdPluginBuilder = testPluginBuild{
		name:   "restish-cmdplugin",
		source: &testPluginBuilder,
		bin:    &testCmdPluginBin,
	}
	testMCPPluginBuilder = testPluginBuild{
		name: "restish-mcp",
		pkg:  "../../cmd/restish-mcp",
		bin:  &testMCPPluginBin,
	}
	testBulkPluginBuilder = testPluginBuild{
		name: "restish-bulk",
		pkg:  "../../cmd/restish-bulk",
		bin:  &testBulkPluginBin,
	}
	testTLSSignerPluginBuilder = testPluginBuild{
		name:   "restish-test-tls-signer",
		source: &testPluginBuilder,
		bin:    &testTLSSignerPluginBin,
	}
	testCSVPluginBuilder = testPluginBuild{
		name: "restish-csv",
		pkg:  "../../cmd/restish-csv",
		bin:  &testCSVPluginBin,
	}
)

// TestMain owns cleanup for lazily built helper plugin binaries.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "restish-cli-test-plugins-*")
	if err == nil {
		testPluginBuildDir = dir
	}

	code := m.Run()

	if testPluginBuildDir != "" {
		_ = os.RemoveAll(testPluginBuildDir)
	}
	os.Exit(code)
}

type testPluginBuild struct {
	once   sync.Once
	name   string
	pkg    string
	source *testPluginBuild
	bin    *string
	out    []byte
	err    error
}

func (b *testPluginBuild) path(t *testing.T, description string) string {
	t.Helper()
	b.once.Do(b.build)
	if *b.bin != "" {
		return *b.bin
	}
	if b.err != nil {
		t.Skipf("%s plugin binary not compiled; skipping %s plugin tests: %v\n%s", description, description, b.err, b.out)
	}
	t.Skipf("%s plugin binary not compiled; skipping %s plugin tests", description, description)
	return ""
}

func (b *testPluginBuild) build() {
	if b.source != nil {
		b.source.once.Do(b.source.build)
		if b.source.err != nil {
			b.out = b.source.out
			b.err = b.source.err
			return
		}
		b.aliasBuiltPlugin(*b.source.bin)
		return
	}

	bin := filepath.Join(testPluginBuildDir, b.name)
	if testPluginBuildDir == "" {
		bin = filepath.Join(os.TempDir(), b.name)
	}
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, b.pkg)
	cmd.Dir = testdataDir()
	b.out, b.err = cmd.CombinedOutput()
	if b.err == nil {
		*b.bin = bin
	}
}

func (b *testPluginBuild) aliasBuiltPlugin(source string) {
	bin := filepath.Join(testPluginBuildDir, b.name)
	if testPluginBuildDir == "" {
		bin = filepath.Join(os.TempDir(), b.name)
	}
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	if err := os.Link(source, bin); err == nil {
		*b.bin = bin
		return
	}
	data, err := os.ReadFile(source)
	if err != nil {
		b.err = err
		return
	}
	if err := os.WriteFile(bin, data, 0o755); err != nil {
		b.out = data
		b.err = err
		return
	}
	*b.bin = bin
}

// testdataDir returns the directory containing testdata/ relative to this file.
func testdataDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

// skipNoPlugin skips the test if the plugin binary wasn't compiled.
func skipNoPlugin(t *testing.T) {
	t.Helper()
	testPluginBuilder.path(t, "test")
}

// skipNoHookPlugin skips the test if the hook plugin binary wasn't compiled.
func skipNoHookPlugin(t *testing.T) {
	t.Helper()
	testHookPluginBuilder.path(t, "hook")
}

func skipNoCmdPlugin(t *testing.T) {
	t.Helper()
	testCmdPluginBuilder.path(t, "command")
}

func skipNoMCPPlugin(t *testing.T) {
	t.Helper()
	testMCPPluginBuilder.path(t, "mcp")
}

func skipNoBulkPlugin(t *testing.T) {
	t.Helper()
	testBulkPluginBuilder.path(t, "bulk")
}

func skipNoTLSSignerPlugin(t *testing.T) {
	t.Helper()
	testTLSSignerPluginBuilder.path(t, "tls-signer")
}

func skipNoCSVPlugin(t *testing.T) {
	t.Helper()
	testCSVPluginBuilder.path(t, "csv")
}

// TestPluginIgnoresPathPlugins verifies that restish-* binaries on PATH are
// not discovered by "plugin list".
func TestPluginIgnoresPathPlugins(t *testing.T) {
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

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	t.Setenv("RSH_CONFIG_DIR", t.TempDir())
	if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
		t.Fatalf("plugin list: %v", err)
	}
	_ = errOut
	if strings.Contains(out.String(), "testplugin") {
		t.Errorf("PATH plugin should not appear in plugin list output, got:\n%s", out.String())
	}
}

// TestPluginDiscoverInPluginDir verifies that a plugin binary in the plugin
// directory is discovered.
func TestPluginDiscoverInPluginDir(t *testing.T) {
	skipNoPlugin(t)

	data, err := os.ReadFile(testPluginBin)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
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

	// Clear PATH to keep the test isolated from the user's shell.
	t.Setenv("PATH", "")

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(pluginsParent, "restish.json")
	if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
		t.Fatalf("plugin list: %v", err)
	}
	if !strings.Contains(out.String(), "testplugin") {
		t.Errorf("expected testplugin from plugin dir, got:\n%s", out.String())
	}
}

func TestPluginRemoveRejectsTraversal(t *testing.T) {
	pluginsParent := t.TempDir()
	pluginDir := filepath.Join(pluginsParent, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(pluginsParent, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	err := c.Run([]string{"restish", "plugin", "remove", "../victim"})
	if err == nil {
		t.Fatal("expected plugin remove to reject traversal")
	}
	if !strings.Contains(err.Error(), "invalid plugin name") {
		t.Fatalf("expected invalid plugin name error, got: %v", err)
	}
	if _, statErr := os.Stat(victim); statErr != nil {
		t.Fatalf("expected victim file to remain, got: %v", statErr)
	}
}

func TestPluginInstallRejectsInvalidPluginBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}

	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "restish-invalid")
	if err := os.WriteFile(source, []byte("#!/bin/sh\necho not-a-manifest\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	err := c.Run([]string{"restish", "plugin", "install", source})
	if err == nil {
		t.Fatal("expected invalid plugin install to fail")
	}

	installed := filepath.Join(pluginsParent, "plugins", "restish-invalid")
	if _, statErr := os.Stat(installed); !os.IsNotExist(statErr) {
		t.Fatalf("expected invalid plugin binary to be removed, got: %v", statErr)
	}
}

func TestPluginInstallWarnsThatPluginsAreTrusted(t *testing.T) {
	skipNoPlugin(t)

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
	if err := c.Run([]string{"restish", "plugin", "install", testPluginBin}); err != nil {
		t.Fatalf("plugin install: %v", err)
	}
	if !strings.Contains(out.String(), "Installed plugin") {
		t.Fatalf("expected install output, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "trusted executables") {
		t.Fatalf("expected trust warning, got:\n%s", errOut.String())
	}
}

// TestPluginListShowsNameVersionHooks verifies that "plugin list" prints
// the name, version, and hooks from the manifest.
func TestPluginListShowsNameVersionHooks(t *testing.T) {
	skipNoPlugin(t)

	pluginsParent := t.TempDir()
	pluginDir := filepath.Join(pluginsParent, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(pluginDir, "restish-testplugin")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	data, _ := os.ReadFile(testPluginBin)
	_ = os.WriteFile(dest, data, 0o755)

	t.Setenv("PATH", "")
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(pluginsParent, "restish.json")
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
	pluginsParent := t.TempDir()
	dir := filepath.Join(pluginsParent, "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a fake plugin that always exits 1.
	badPlugin := filepath.Join(dir, "restish-bad")
	if runtime.GOOS == "windows" {
		t.Skip("shell script test not supported on Windows")
	}
	if err := os.WriteFile(badPlugin, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", "")
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(pluginsParent, "restish.json")

	// Should not return an error — broken plugins are skipped.
	if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
		t.Fatalf("plugin list with bad plugin: %v", err)
	}
	// Warning should appear on stderr.
	if !strings.Contains(errOut.String(), "warning") {
		t.Errorf("expected warning for bad plugin on stderr, got:\n%s", errOut.String())
	}
	if strings.Contains(errOut.String(), "run with -v for details") {
		t.Errorf("did not expect misleading -v hint, got:\n%s", errOut.String())
	}
	// Bad plugin must not appear in stdout list.
	if strings.Contains(out.String(), "restish-bad") {
		t.Errorf("bad plugin should not appear in list:\n%s", out.String())
	}
}
