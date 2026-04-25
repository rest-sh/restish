package cli_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/rest-sh/restish/v2/internal/plugin"
	"github.com/rest-sh/restish/v2/internal/spec"
)

// installHookPlugin copies testHookPluginBin into pluginsParent/plugins/ and
// sets RSH_CONFIG_DIR to pluginsParent so Run()'s Discover() finds it.
// The plugin dir is returned.
func installHookPlugin(t *testing.T) string {
	t.Helper()
	skipNoHookPlugin(t)

	data, err := os.ReadFile(testHookPluginBin)
	if err != nil {
		t.Fatalf("read hook plugin: %v", err)
	}

	pluginsParent := t.TempDir()
	pluginDir := filepath.Join(pluginsParent, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(pluginDir, "restish-hookplugin")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		t.Fatalf("write hook plugin: %v", err)
	}

	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	// Clear PATH so no other plugins from the environment interfere.
	t.Setenv("PATH", "")

	return pluginDir
}

func installCSVPlugin(t *testing.T) string {
	t.Helper()
	skipNoCSVPlugin(t)

	data, err := os.ReadFile(testCSVPluginBin)
	if err != nil {
		t.Fatalf("read csv plugin: %v", err)
	}

	pluginsParent := t.TempDir()
	pluginDir := filepath.Join(pluginsParent, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(pluginDir, "restish-csv")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		t.Fatalf("write csv plugin: %v", err)
	}

	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	t.Setenv("PATH", "")

	return pluginDir
}

// hookJSONServer starts a test server that always responds with status and JSON body.
func hookJSONServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestAuthHookPlugin verifies that an auth hook plugin adds the Authorization
// header to requests made to a registered API.
func TestAuthHookPlugin(t *testing.T) {
	installHookPlugin(t)

	var mu sync.Mutex
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	cfg := fmt.Sprintf(`{"apis":{"testapi":{"base_url":%q,"profiles":{"default":{}}}}}`, srv.URL)
	c, _, _ := newTestCLI()
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "testapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	got := capturedAuth
	mu.Unlock()

	if got != "Bearer hook-token" {
		t.Errorf("Authorization header: got %q, want %q", got, "Bearer hook-token")
	}
}

func TestAuthHookPluginWithoutProfiles(t *testing.T) {
	installHookPlugin(t)

	var mu sync.Mutex
	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	cfg := fmt.Sprintf(`{"apis":{"testapi":{"base_url":%q}}}`, srv.URL)
	c, _, _ := newTestCLI()
	c.Hooks().ConfigPath = writeAPIConfig(t, cfg)

	if err := c.Run([]string{"restish", "get", "testapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	got := capturedAuth
	mu.Unlock()

	if got != "Bearer hook-token" {
		t.Errorf("Authorization header: got %q, want %q", got, "Bearer hook-token")
	}
}

func TestHookPluginsRedactSecretRequestHeadersByDefault(t *testing.T) {
	installHookPlugin(t)
	t.Setenv("RSH_HOOK_EXPECT_SECRET_HEADERS", "redacted")
	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "")

	srv := hookJSONServer(t, 200, `{"ok":true}`)
	c, _, _ := newTestCLI()
	c.Hooks().ConfigPath = writeAPIConfig(t, hookSecretHeaderConfig(srv.URL))

	if err := c.Run([]string{"restish", "get", "testapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHookPluginsPreserveSecretRequestHeadersWhenOptedIn(t *testing.T) {
	installHookPlugin(t)
	t.Setenv("RSH_HOOK_NEEDS_AUTH_SECRETS", "1")
	t.Setenv("RSH_HOOK_EXPECT_SECRET_HEADERS", "preserved")
	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "")

	srv := hookJSONServer(t, 200, `{"ok":true}`)
	c, _, _ := newTestCLI()
	c.Hooks().ConfigPath = writeAPIConfig(t, hookSecretHeaderConfig(srv.URL))

	if err := c.Run([]string{"restish", "get", "testapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func hookSecretHeaderConfig(baseURL string) string {
	return fmt.Sprintf(`{
		"apis": {
			"testapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"headers": [
							"Authorization: Bearer original",
							"Cookie: session=abc",
							"Proxy-Authorization: Basic proxy"
						]
					}
				}
			}
		}
	}`, baseURL)
}

// TestRequestMiddlewarePlugin verifies that a request-middleware hook plugin
// adds a header to the outbound request.
func TestRequestMiddlewarePlugin(t *testing.T) {
	installHookPlugin(t)

	var mu sync.Mutex
	var capturedTrace string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedTrace = r.Header.Get("X-Trace-Id")
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	c, _, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	got := capturedTrace
	mu.Unlock()

	if got != "hook-trace-123" {
		t.Errorf("X-Trace-Id: got %q, want %q", got, "hook-trace-123")
	}
}

// TestResponseMiddlewarePluginModify verifies that a response-middleware plugin
// can add a field to the response body.
func TestResponseMiddlewarePluginModify(t *testing.T) {
	installHookPlugin(t)
	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "")

	srv := hookJSONServer(t, 200, `{"hello":"world"}`)
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "plugin_added") {
		t.Errorf("expected plugin_added in output, got:\n%s", out.String())
	}
}

// TestResponseMiddlewarePluginDrop verifies that a response-middleware plugin
// can suppress all output by returning {"drop":true}.
func TestResponseMiddlewarePluginDrop(t *testing.T) {
	installHookPlugin(t)
	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "drop")

	srv := hookJSONServer(t, 200, `{"hello":"world"}`)
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out.Len() != 0 {
		t.Errorf("expected no output after drop, got:\n%s", out.String())
	}
}

// TestResponseMiddlewarePluginFollow verifies that a response-middleware plugin
// can redirect Restish to issue a follow-up request.
func TestResponseMiddlewarePluginFollow(t *testing.T) {
	installHookPlugin(t)

	// Second server: the follow target.
	follow := hookJSONServer(t, 200, `{"from":"follow"}`)

	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "follow:"+follow.URL)

	// First server: the initial request target.
	first := hookJSONServer(t, 200, `{"from":"first"}`)
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", first.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "follow") {
		t.Errorf("expected follow-target response in output, got:\n%s", out.String())
	}
	if strings.Contains(out.String(), "first") {
		t.Errorf("expected first-target response to be replaced, got:\n%s", out.String())
	}
}

// TestFormatterPlugin verifies that a formatter hook plugin is invoked when its
// declared format name is requested via -o and its raw output is passed through.
func TestFormatterPlugin(t *testing.T) {
	installHookPlugin(t)

	srv := hookJSONServer(t, 200, `{"hello":"world"}`)
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", "-o", "hookformat", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "HOOK FORMATTED") {
		t.Errorf("expected HOOK FORMATTED in output, got:\n%s", out.String())
	}
}

// TestFormatterPluginNotInvokedWithoutFlag verifies that the formatter plugin is
// not invoked when a different (or no) output format is requested.
func TestFormatterPluginNotInvokedWithoutFlag(t *testing.T) {
	installHookPlugin(t)

	srv := hookJSONServer(t, 200, `{"hello":"world"}`)
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	// No -o flag: default output should NOT contain plugin text.
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out.String(), "HOOK FORMATTED") {
		t.Errorf("expected default output, got plugin output:\n%s", out.String())
	}
}

func TestCSVFormatterPlugin(t *testing.T) {
	installCSVPlugin(t)

	srv := hookJSONServer(t, 200, `[{"id":1,"name":"alpha"},{"id":2,"name":"beta","active":true}]`)
	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", "-o", "csv", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Join([]string{
		"active,id,name",
		",1,alpha",
		"true,2,beta",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("csv output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestCSVFormatterPluginPagination(t *testing.T) {
	installCSVPlugin(t)

	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		pages := map[string]struct {
			body string
			next string
		}{
			"":  {`[{"id":1,"name":"alpha"}]`, "https://api.example.com/items?page=2"},
			"2": {`[{"id":2,"name":"beta"}]`, ""},
		}
		p := pages[r.URL.Query().Get("page")]
		headers := http.Header{"Content-Type": []string{"application/json"}}
		if p.next != "" {
			headers.Set("Link", `<`+p.next+`>; rel="next"`)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     headers,
			Body:       io.NopCloser(strings.NewReader(p.body)),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "-o", "csv"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Join([]string{
		"id,name",
		"1,alpha",
		"2,beta",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("csv paginated output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestCSVFormatterPluginNDJSONStream(t *testing.T) {
	installCSVPlugin(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		fmt.Fprintln(w, `{"id":1,"name":"alpha"}`)
		fmt.Fprintln(w, `{"id":2,"name":"beta"}`)
	}))
	t.Cleanup(srv.Close)

	c, out, _ := newTestCLI()
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "get", srv.URL, "-o", "csv"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Join([]string{
		"id,name",
		"1,alpha",
		"2,beta",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("csv stream output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestLoaderPlugin verifies that a loader hook plugin can detect a custom
// content type and return a valid OpenAPI spec that is correctly parsed.
func TestLoaderPlugin(t *testing.T) {
	skipNoHookPlugin(t)

	pl := spec.PluginLoader{
		PluginPath:   testHookPluginBin,
		PluginName:   "hookplugin",
		ContentTypes: []string{"application/x-hook-api"},
	}

	// Detect should return true for the declared content type.
	if !pl.Detect("application/x-hook-api", nil) {
		t.Fatal("Detect: expected true for application/x-hook-api")
	}
	if pl.Detect("application/json", nil) {
		t.Fatal("Detect: expected false for application/json")
	}

	// Load should return a valid APISpec with an OpenAPI document.
	apiSpec, err := pl.Load([]byte(`{"some":"body"}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if apiSpec == nil {
		t.Fatal("Load: expected non-nil APISpec")
	}
	if apiSpec.Document == nil {
		t.Fatal("Load: expected non-nil Document")
	}
}

// TestLoaderPluginRegistration verifies that loader plugins discovered at
// startup are registered in the CLI's loader list.
func TestLoaderPluginRegistration(t *testing.T) {
	installHookPlugin(t)

	// Verify the plugin declares the expected content type.
	plugins := plugin.Discover(plugin.DefaultPluginDir(), nil, "", nil)
	if len(plugins) == 0 {
		t.Fatal("no plugins discovered")
	}
	var found bool
	for _, p := range plugins {
		for _, ct := range p.Manifest.LoaderContentTypes {
			if ct == "application/x-hook-api" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected application/x-hook-api in loader content types")
	}
}
