package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
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
	c.ConfigPath = writeAPIConfig(t, cfg)

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
	c.ConfigPath = t.TempDir() + "/restish.json"
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
	c.ConfigPath = t.TempDir() + "/restish.json"
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
	c.ConfigPath = t.TempDir() + "/restish.json"
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
	c.ConfigPath = t.TempDir() + "/restish.json"
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
