package cli

import (
	"testing"

	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
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
