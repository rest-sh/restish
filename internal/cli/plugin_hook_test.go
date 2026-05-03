//go:build integration

package cli_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	pluginsParent, pluginDir := installSharedPlugin(t, "hook", testHookPluginBin, "restish-hookplugin")

	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	// Clear PATH so no other plugins from the environment interfere.
	t.Setenv("PATH", "")

	return pluginDir
}

func installCSVPlugin(t *testing.T) string {
	t.Helper()
	skipNoCSVPlugin(t)

	pluginsParent, pluginDir := installSharedPlugin(t, "csv", testCSVPluginBin, "restish-csv")

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
	c, _, _ := newTestCLI(t)
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
	c, _, _ := newTestCLI(t)
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
	t.Setenv("RSH_HOOK_EXPECT_SECRET_QUERY", "redacted")
	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "")

	srv := hookJSONServer(t, 200, `{"ok":true}`)
	c, _, _ := newTestCLI(t)
	c.Hooks().PluginManifestCachePath = filepath.Join(t.TempDir(), "plugin-manifest.cbor")
	c.Hooks().ConfigPath = writeAPIConfig(t, hookSecretHeaderConfig(srv.URL))

	if err := c.Run([]string{"restish", "get", "testapi/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHookPluginsPreserveSecretRequestHeadersWhenOptedIn(t *testing.T) {
	installHookPlugin(t)
	t.Setenv("RSH_HOOK_NEEDS_AUTH_SECRETS", "1")
	t.Setenv("RSH_HOOK_EXPECT_SECRET_HEADERS", "preserved")
	t.Setenv("RSH_HOOK_EXPECT_SECRET_QUERY", "preserved")
	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "")

	srv := hookJSONServer(t, 200, `{"ok":true}`)
	c, _, _ := newTestCLI(t)
	c.Hooks().PluginManifestCachePath = filepath.Join(t.TempDir(), "plugin-manifest.cbor")
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
								"Proxy-Authorization: Basic proxy",
								"X-Api-Key: api-secret",
								"X-Auth-Token: auth-secret",
								"X-Secret: secret"
							],
							"query": [
								"api_key=secret",
								"token=secret",
								"view=summary"
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

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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

func TestResponseMiddlewarePluginFollowWithHeadersAndBody(t *testing.T) {
	installHookPlugin(t)

	var gotMethod string
	var gotToken string
	var gotContentType string
	var gotBody map[string]any
	follow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotToken = r.Header.Get("X-Follow-Token")
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode follow body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"from":"follow"}`)
	}))
	t.Cleanup(follow.Close)

	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "follow-body:"+follow.URL)

	first := hookJSONServer(t, 200, `{"from":"first"}`)
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	if err := c.Run([]string{"restish", "get", first.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("follow method = %q, want POST", gotMethod)
	}
	if gotToken != "hook-token" {
		t.Fatalf("X-Follow-Token = %q, want hook-token", gotToken)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["from"] != "plugin" {
		t.Fatalf("follow body = %#v, want from=plugin", gotBody)
	}
	if !strings.Contains(out.String(), "follow") {
		t.Errorf("expected follow-target response in output, got:\n%s", out.String())
	}
}

func TestResponseMiddlewarePluginFollowCrossHostStripsProfileQueryCredentials(t *testing.T) {
	installHookPlugin(t)

	var gotQuery string
	var gotAuthorization string
	var gotAPIKey string
	var gotTrace string
	var gotMethod string
	var gotBody map[string]any
	follow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotAuthorization = r.Header.Get("Authorization")
		gotAPIKey = r.Header.Get("X-Api-Key")
		gotTrace = r.Header.Get("X-Trace-Id")
		gotMethod = r.Method
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode follow body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"from":"follow"}`)
	}))
	t.Cleanup(follow.Close)

	t.Setenv("RSH_HOOK_RM_BEHAVIOR", "follow-body:"+follow.URL)
	t.Setenv("RSH_HOOK_REQUEST_AUTH", "1")

	first := hookJSONServer(t, 200, `{"from":"first"}`)
	c, _, errOut := newTestCLI(t)
	cfgPath := c.Hooks().ConfigPath
	if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf(`{
  "apis": {
    "follow": {
      "base_url": %q,
      "profiles": {
        "default": {
          "headers": ["Authorization: Bearer profile-token", "X-Api-Key: profile-key"],
          "query": ["api_key=secret", "token=secret", "view=summary"]
        }
      }
    }
  }
}`, follow.URL)), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := c.Run([]string{"restish", "get", first.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(errOut.String(), "stripping credentials") {
		t.Fatalf("expected stripping credentials warning, got:\n%s", errOut.String())
	}

	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("ParseQuery(%q): %v", gotQuery, err)
	}
	if got := values.Get("api_key"); got != "" {
		t.Fatalf("api_key query leaked to follow target: %q", got)
	}
	if got := values.Get("token"); got != "" {
		t.Fatalf("token query leaked to follow target: %q", got)
	}
	if got := values.Get("view"); got != "summary" {
		t.Fatalf("non-sensitive query = %q, want summary (raw %q)", got, gotQuery)
	}
	if gotAuthorization != "" {
		t.Fatalf("Authorization leaked to follow target: %q", gotAuthorization)
	}
	if gotAPIKey != "" {
		t.Fatalf("X-Api-Key leaked to follow target: %q", gotAPIKey)
	}
	if gotTrace != "" {
		t.Fatalf("request middleware ran on cross-host follow, X-Trace-Id=%q", gotTrace)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("follow method = %q, want POST", gotMethod)
	}
	if gotBody["from"] != "plugin" {
		t.Fatalf("follow body = %#v, want from=plugin", gotBody)
	}
}

// TestFormatterPlugin verifies that a formatter hook plugin is invoked when its
// declared format name is requested via -o and its raw output is passed through.
func TestFormatterPlugin(t *testing.T) {
	installHookPlugin(t)

	srv := hookJSONServer(t, 200, `{"hello":"world"}`)
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	// No -o flag: default output should NOT contain plugin text.
	if err := c.Run([]string{"restish", "get", srv.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(out.String(), "HOOK FORMATTED") {
		t.Errorf("expected default output, got plugin output:\n%s", out.String())
	}
}

func TestFormatterPluginRequiresDeclaredHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test uses a POSIX shell plugin fixture")
	}

	pluginsParent := t.TempDir()
	pluginDir := filepath.Join(pluginsParent, "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(pluginDir, "restish-rogue")
	script := `#!/bin/sh
if [ "$1" = "--rsh-plugin-manifest" ]; then
  printf '%s\n' '{"name":"rogue","restish_api_version":2,"formatter_names":["rogue"]}'
  exit 0
fi
printf 'ROGUE FORMATTER\n'
`
	if err := os.WriteFile(pluginPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	t.Setenv("PATH", "")

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
	useJSONResponse(c, 200, `{"ok":true}`)
	err := c.Run([]string{"restish", "get", "-o", "rogue", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected undeclared formatter to stay unavailable")
	}
	if !strings.Contains(err.Error(), `unknown output format "rogue"`) {
		t.Fatalf("unexpected error for undeclared formatter: %v", err)
	}
}

func TestCSVFormatterPlugin(t *testing.T) {
	installCSVPlugin(t)

	srv := hookJSONServer(t, 200, `[{"id":1,"name":"alpha"},{"id":2,"name":"beta","active":true}]`)
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = sharedPluginConfigPath(t)
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

func TestLoaderPluginReceivesSourceMetadata(t *testing.T) {
	skipNoHookPlugin(t)
	t.Setenv("RSH_HOOK_EXPECT_LOADER_METADATA", "1")

	pl := spec.PluginLoader{
		PluginPath:   testHookPluginBin,
		PluginName:   "hookplugin",
		ContentTypes: []string{"application/x-hook-api"},
	}
	apiSpec, err := pl.LoadWithOptions([]byte(`{"some":"body"}`), spec.LoadOptions{
		ContentType: "application/x-hook-api",
		SourceURL:   "https://example.test/openapi.hook",
		LocalPath:   "/tmp/openapi.hook",
	})
	if err != nil {
		t.Fatalf("LoadWithOptions: %v", err)
	}
	if apiSpec == nil || apiSpec.Document == nil {
		t.Fatal("LoadWithOptions: expected parsed APISpec")
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
