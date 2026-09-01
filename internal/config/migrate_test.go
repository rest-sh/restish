package config

import (
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/config"
)

func TestRenderMigratedConfigLeadsWithJSONCModeLine(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"example": {BaseURL: "https://api.example.com"},
		},
	}
	backupDir := "/tmp/restish.bak.v1"

	data, err := renderMigratedConfig(cfg, backupDir)
	if err != nil {
		t.Fatalf("renderMigratedConfig: %v", err)
	}
	out := string(data)

	if !strings.HasPrefix(out, "// -*- mode: jsonc -*-\n") {
		t.Fatalf("expected JSONC mode line prefix, got:\n%s", out)
	}
	if !strings.Contains(out, "// Migrated from Restish v1.\n") {
		t.Fatalf("expected migration header, got:\n%s", out)
	}
	if !strings.Contains(out, backupDir) {
		t.Fatalf("expected backup path %q in header, got:\n%s", backupDir, out)
	}

	// Restish loads the migrated payload as JSONC.
	loaded, err := config.ParseConfigBytes("restish.json", data)
	if err != nil {
		t.Fatalf("ParseConfigBytes: %v", err)
	}
	api := loaded.APIs["example"]
	if api == nil || api.BaseURL != "https://api.example.com" {
		t.Fatalf("parsed config missing example API: %+v", loaded.APIs)
	}
}
