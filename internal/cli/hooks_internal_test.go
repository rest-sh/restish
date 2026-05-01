package cli

import (
	"net/http"
	"strings"
	"testing"

	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

func TestIndexPluginsByHook(t *testing.T) {
	plugins := []internalplugin.Plugin{
		{Manifest: internalplugin.Manifest{Name: "auth-one", Hooks: []string{"auth"}}},
		{Manifest: internalplugin.Manifest{Name: "combo", Hooks: []string{"auth", "command"}}},
	}

	indexed := indexPluginsByHook(plugins)
	if len(indexed["auth"]) != 2 {
		t.Fatalf("auth hook count = %d, want 2", len(indexed["auth"]))
	}
	if len(indexed["command"]) != 1 {
		t.Fatalf("command hook count = %d, want 1", len(indexed["command"]))
	}
	if got := indexed["command"][0].Manifest.Name; got != "combo" {
		t.Fatalf("command hook plugin = %q, want combo", got)
	}
}

func TestIndexAuthPluginsByAPI(t *testing.T) {
	plugins := []internalplugin.Plugin{
		{Manifest: internalplugin.Manifest{Name: "global", Hooks: []string{"auth"}}},
		{Manifest: internalplugin.Manifest{Name: "scoped", Hooks: []string{"auth"}, AuthAPINames: []string{"api1", "api2"}}},
	}
	global, byAPI := indexAuthPluginsByAPI(plugins)
	if len(global) != 1 || global[0].Manifest.Name != "global" {
		t.Fatalf("global plugins = %#v", global)
	}
	if len(byAPI["api1"]) != 1 || byAPI["api1"][0].Manifest.Name != "scoped" {
		t.Fatalf("api1 plugins = %#v", byAPI["api1"])
	}
	if len(byAPI["api2"]) != 1 || byAPI["api2"][0].Manifest.Name != "scoped" {
		t.Fatalf("api2 plugins = %#v", byAPI["api2"])
	}
}

func TestPluginDeclaresHook(t *testing.T) {
	manifest := internalplugin.Manifest{
		Name:           "rogue",
		Hooks:          []string{"auth"},
		FormatterNames: []string{"rogue"},
	}
	if !pluginDeclaresHook(manifest, "auth") {
		t.Fatal("expected auth hook to be declared")
	}
	if pluginDeclaresHook(manifest, "formatter") {
		t.Fatal("formatter_names must not imply formatter hook declaration")
	}
}

func TestHookRequestForPluginIncludesBodyHashAndOptInBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://api.example.com/items?token=secret", strings.NewReader(`{"name":"alpha"}`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer secret")

	redacted := hookRequestForPlugin(req, internalplugin.Plugin{Manifest: internalplugin.Manifest{Name: "hash-only"}})
	if redacted.BodySHA256 == "" {
		t.Fatal("expected request body hash")
	}
	if len(redacted.Body) != 0 {
		t.Fatalf("expected body bytes omitted without opt-in, got %q", redacted.Body)
	}
	if got := firstHeaderValue(redacted.Headers, "Authorization"); got != "<redacted>" {
		t.Fatalf("Authorization header = %q, want redacted", got)
	}
	if strings.Contains(redacted.URI, "secret") {
		t.Fatalf("URI was not redacted: %s", redacted.URI)
	}

	withBody := hookRequestForPlugin(req, internalplugin.Plugin{Manifest: internalplugin.Manifest{
		Name:             "signer",
		RequiredFeatures: []string{pluginwire.FeatureRequestFinalBody},
	}})
	if string(withBody.Body) != `{"name":"alpha"}` {
		t.Fatalf("body = %q, want original bytes", withBody.Body)
	}
	if got := firstHeaderValue(withBody.Headers, "Authorization"); got != "<redacted>" {
		t.Fatalf("Authorization header = %q, want redacted", got)
	}
}

func TestApplyRequestUpdateNilDeletesHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/items", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("X-Remove", "old")
	req.Header.Set("X-Keep", "old")

	applyRequestUpdate(req, &pluginwire.HookRequestHeaderUpdate{Headers: map[string]any{
		"X-Remove": nil,
		"X-Keep":   "new",
	}})

	if got := req.Header.Get("X-Remove"); got != "" {
		t.Fatalf("X-Remove = %q, want deleted", got)
	}
	if got := req.Header.Get("X-Keep"); got != "new" {
		t.Fatalf("X-Keep = %q, want new", got)
	}
}

func firstHeaderValue(headers map[string][]string, name string) string {
	values := headers[name]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
