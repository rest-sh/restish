//go:build integration

package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

var (
	testPluginBuildDir    string
	sharedPluginInstallMu sync.Mutex
)

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
		testPluginManifestCachePath = filepath.Join(dir, "plugin-manifest.cbor")
	}

	if shouldPrebuildTestPlugins() {
		prebuildTestPlugins()
	}

	code := m.Run()

	if testPluginBuildDir != "" {
		_ = os.RemoveAll(testPluginBuildDir)
	}
	os.Exit(code)
}

func shouldPrebuildTestPlugins() bool {
	run := ""
	if f := flag.Lookup("test.run"); f != nil {
		run = f.Value.String()
	}
	return run == "" || run == "." || run == ".*"
}

func prebuildTestPlugins() {
	sourceBuilders := []*testPluginBuild{
		&testPluginBuilder,
		&testMCPPluginBuilder,
		&testBulkPluginBuilder,
		&testCSVPluginBuilder,
	}
	var wg sync.WaitGroup
	for _, builder := range sourceBuilders {
		wg.Add(1)
		go func(b *testPluginBuild) {
			defer wg.Done()
			b.once.Do(b.build)
		}(builder)
	}
	wg.Wait()

	for _, builder := range []*testPluginBuild{
		&testHookPluginBuilder,
		&testCmdPluginBuilder,
		&testTLSSignerPluginBuilder,
	} {
		builder.once.Do(builder.build)
	}
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

func installSharedPlugin(t *testing.T, dirName, source, name string) (string, string) {
	t.Helper()

	if testPluginBuildDir == "" {
		pluginsParent := t.TempDir()
		pluginDir := filepath.Join(pluginsParent, "plugins")
		copyTestPlugin(t, source, filepath.Join(pluginDir, name))
		return pluginsParent, pluginDir
	}

	pluginsParent := filepath.Join(testPluginBuildDir, "installed", dirName)
	pluginDir := filepath.Join(pluginsParent, "plugins")
	dest := filepath.Join(pluginDir, name)
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}

	sharedPluginInstallMu.Lock()
	defer sharedPluginInstallMu.Unlock()
	if _, err := os.Stat(dest); err == nil {
		return pluginsParent, pluginDir
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat shared plugin: %v", err)
	}

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, dest); err == nil {
		return pluginsParent, pluginDir
	}
	copyTestPlugin(t, source, dest)
	return pluginsParent, pluginDir
}

func sharedPluginConfigPath(t *testing.T) string {
	t.Helper()
	if configDir := os.Getenv("RSH_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, "restish.json")
	}
	return filepath.Join(t.TempDir(), "restish.json")
}

func copyTestPlugin(t *testing.T, source, dest string) {
	t.Helper()

	if runtime.GOOS == "windows" && filepath.Ext(dest) == "" {
		dest += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
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
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	err := c.Run([]string{"restish", "plugin", "install", "--yes", source})
	if err == nil {
		t.Fatal("expected invalid plugin install to fail")
	}

	installed := filepath.Join(pluginsParent, "plugins", "restish-invalid")
	if _, statErr := os.Stat(installed); !os.IsNotExist(statErr) {
		t.Fatalf("expected invalid plugin binary to be removed, got: %v", statErr)
	}
}

func TestPluginInstallDeclineDoesNotExecuteManifest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}

	sourceDir := t.TempDir()
	marker := filepath.Join(sourceDir, "executed")
	source := filepath.Join(sourceDir, "restish-decline")
	script := "#!/bin/sh\nif [ \"$1\" = \"--rsh-plugin-manifest\" ]; then echo ran > " + marker + "; echo '{\"name\":\"decline\",\"restish_api_version\":1}'; fi\n"
	if err := os.WriteFile(source, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	err := c.Run([]string{"restish", "plugin", "install", source})
	if err == nil {
		t.Fatal("expected declined plugin install to fail")
	}
	if !strings.Contains(errOut.String(), "Inspect and trust this plugin?") {
		t.Fatalf("expected trust prompt, got:\n%s", errOut.String())
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("plugin manifest command should not run before confirmation, stat: %v", statErr)
	}
}

func TestPluginInstallRejectsManifestNameTraversal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}

	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "restish-traverse")
	script := "#!/bin/sh\nif [ \"$1\" = \"--rsh-plugin-manifest\" ]; then echo '{\"name\":\"../victim\",\"restish_api_version\":1}'; exit 0; fi\n"
	if err := os.WriteFile(source, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	victim := filepath.Join(pluginsParent, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	err := c.Run([]string{"restish", "plugin", "install", "--yes", source})
	if err == nil {
		t.Fatal("expected traversal manifest name to fail")
	}
	if !strings.Contains(err.Error(), "manifest name") {
		t.Fatalf("expected manifest name error, got: %v", err)
	}
	data, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("read victim: %v", err)
	}
	if string(data) != "keep" {
		t.Fatalf("victim was modified: %q", data)
	}
}

func TestPluginInstallWarnsThatPluginsAreTrusted(t *testing.T) {
	skipNoPlugin(t)

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, out, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	if err := c.Run([]string{"restish", "plugin", "install", "--yes", testPluginBin}); err != nil {
		t.Fatalf("plugin install: %v", err)
	}
	if !strings.Contains(out.String(), "Installed plugin") {
		t.Fatalf("expected install output, got:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "trusted executables") {
		t.Fatalf("expected trust warning, got:\n%s", errOut.String())
	}
}

func TestPluginInstallRequiresYesNonInteractive(t *testing.T) {
	skipNoPlugin(t)

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	err := c.Run([]string{"restish", "plugin", "install", testPluginBin})
	if err == nil {
		t.Fatal("expected plugin install without --yes to fail noninteractively")
	}
	if !strings.Contains(err.Error(), "confirmation required") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(errOut.String(), "Capabilities:") {
		t.Fatalf("manifest should not be read before confirmation, got:\n%s", errOut.String())
	}
	if !strings.Contains(errOut.String(), "Resolved path:") {
		t.Fatalf("expected source summary before confirmation, got:\n%s", errOut.String())
	}
}

func TestPluginInstallPromptsAndAcceptsConfirmation(t *testing.T) {
	skipNoPlugin(t)

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, out, errOut := newTestCLI(t)
	c.Hooks().PassReader = strings.NewReader("y\n")
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	if err := c.Run([]string{"restish", "plugin", "install", testPluginBin}); err != nil {
		t.Fatalf("plugin install with confirmation: %v", err)
	}
	if !strings.Contains(errOut.String(), "Inspect and trust this plugin?") {
		t.Fatalf("expected trust prompt, got:\n%s", errOut.String())
	}
	if !strings.Contains(out.String(), "Installed plugin") {
		t.Fatalf("expected install output, got:\n%s", out.String())
	}
}

func TestPluginInstallFromPath(t *testing.T) {
	skipNoPlugin(t)

	pathDir := t.TempDir()
	pathPlugin := filepath.Join(pathDir, "restish-testplugin")
	if runtime.GOOS == "windows" {
		pathPlugin += ".exe"
	}
	data, err := os.ReadFile(testPluginBin)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	if err := os.WriteFile(pathPlugin, data, 0o755); err != nil {
		t.Fatalf("write path plugin: %v", err)
	}

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	t.Setenv("PATH", pathDir)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	if err := c.Run([]string{"restish", "plugin", "install", "--yes", "restish-testplugin"}); err != nil {
		t.Fatalf("plugin install from PATH: %v", err)
	}
	if !strings.Contains(out.String(), "Installed plugin restish-testplugin") {
		t.Fatalf("expected install output, got:\n%s", out.String())
	}

	installed := filepath.Join(pluginsParent, "plugins", filepath.Base(pathPlugin))
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("expected installed plugin at %s: %v", installed, err)
	}
}

func TestPluginInstallFromGitHubShorthand(t *testing.T) {
	skipNoPlugin(t)

	assetName := "restish-testplugin_v1.2.3_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archive := tarGzPlugin(t, testPluginBin, "restish-testplugin")
	var sawGitHubAPI, sawDownload bool

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.URL.Host == "api.github.com" && r.URL.Path == "/repos/acme/tools/releases/latest":
			sawGitHubAPI = true
			body := `{"assets":[{"name":"` + assetName + `","browser_download_url":"https://downloads.example/` + assetName + `"}]}`
			return testHTTPResponse(200, "application/json", []byte(body)), nil
		case r.URL.Host == "downloads.example" && r.URL.Path == "/"+assetName:
			sawDownload = true
			return testHTTPResponse(200, "application/gzip", archive), nil
		default:
			return testHTTPResponse(404, "text/plain", []byte("not found")), nil
		}
	})

	if err := c.Run([]string{"restish", "plugin", "install", "--yes", "acme/tools:testplugin"}); err != nil {
		t.Fatalf("plugin install from github shorthand: %v", err)
	}
	if !sawGitHubAPI || !sawDownload {
		t.Fatalf("expected GitHub API and download calls, got api=%v download=%v", sawGitHubAPI, sawDownload)
	}
	if !strings.Contains(out.String(), "Installed plugin restish-testplugin") {
		t.Fatalf("expected install output, got:\n%s", out.String())
	}
	installed := filepath.Join(pluginsParent, "plugins", "restish-testplugin")
	if runtime.GOOS == "windows" {
		installed += ".exe"
	}
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("expected installed plugin at %s: %v", installed, err)
	}
}

func TestPluginInstallFromURLArchive(t *testing.T) {
	skipNoPlugin(t)

	archiveName := "restish-testplugin_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	archive := tarGzPlugin(t, testPluginBin, "restish-testplugin")

	pluginsParent := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "downloads.example" && r.URL.Path == "/"+archiveName {
			return testHTTPResponse(200, "application/gzip", archive), nil
		}
		return testHTTPResponse(404, "text/plain", []byte("not found")), nil
	})

	if err := c.Run([]string{"restish", "plugin", "install", "--yes", "https://downloads.example/" + archiveName}); err != nil {
		t.Fatalf("plugin install from URL archive: %v", err)
	}
	if !strings.Contains(out.String(), "Installed plugin restish-testplugin") {
		t.Fatalf("expected install output, got:\n%s", out.String())
	}
}

func tarGzPlugin(t *testing.T, source, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read plugin: %v", err)
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(data)),
	}); err != nil {
		t.Fatalf("write tar header: %v", err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write tar data: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func testHTTPResponse(status int, contentType string, body []byte) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	if contentType != "" {
		resp.Header.Set("Content-Type", contentType)
	}
	return resp
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

func TestPluginListShowsCommandNamesAndJSON(t *testing.T) {
	skipNoPlugin(t)

	pluginsParent := t.TempDir()
	pluginDir := filepath.Join(pluginsParent, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(pluginDir, "restish-cmdplugin")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	data, _ := os.ReadFile(testPluginBin)
	_ = os.WriteFile(dest, data, 0o755)

	t.Setenv("PATH", "")
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(pluginsParent, "restish.json")
	if err := c.Run([]string{"restish", "plugin", "list"}); err != nil {
		t.Fatalf("plugin list: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "commands: greet, fetch") {
		t.Fatalf("expected command names in plugin list, got:\n%s", got)
	}

	c, out, _ = newTestCLI(t)
	c.Hooks().ConfigPath = filepath.Join(pluginsParent, "restish.json")
	if err := c.Run([]string{"restish", "plugin", "list", "--json"}); err != nil {
		t.Fatalf("plugin list --json: %v", err)
	}
	if !strings.Contains(out.String(), `"name": "cmdplugin"`) ||
		!strings.Contains(out.String(), `"commands":`) ||
		!strings.Contains(out.String(), `"greet"`) {
		t.Fatalf("expected machine-readable command list, got:\n%s", out.String())
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
