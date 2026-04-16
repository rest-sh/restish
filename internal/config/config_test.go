package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/config"
)

// writeConfig writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "restish.json")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return path
}

func TestLoad_Empty(t *testing.T) {
	path := writeConfig(t, `{}`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restish.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("missing file should not return an error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for missing file")
	}
}

func TestLoad_JSONC_Comments(t *testing.T) {
	path := writeConfig(t, `{
		// This is a comment
		"apis": {
			/* block comment */
			"myapi": {
				"base_url": "https://api.example.com"
			}
		}
	}`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("JSONC with comments should parse without error, got: %v", err)
	}
	if cfg.APIs["myapi"] == nil {
		t.Fatal("expected myapi to be loaded")
	}
	if cfg.APIs["myapi"].BaseURL != "https://api.example.com" {
		t.Errorf("unexpected base_url: %q", cfg.APIs["myapi"].BaseURL)
	}
}

func TestLoad_ValidAPIs(t *testing.T) {
	path := writeConfig(t, `{
		"apis": {
			"github": { "base_url": "https://api.github.com" },
			"stripe": { "base_url": "https://api.stripe.com" }
		}
	}`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.APIs) != 2 {
		t.Fatalf("expected 2 APIs, got %d", len(cfg.APIs))
	}
	if cfg.APIs["github"].BaseURL != "https://api.github.com" {
		t.Errorf("unexpected github base_url: %q", cfg.APIs["github"].BaseURL)
	}
}

func TestLoad_ProfileTLSSignerParams(t *testing.T) {
	path := writeConfig(t, `{
		"apis": {
			"demo": {
				"profiles": {
					"default": {
						"tls_signer": "pkcs11",
						"tls_signer_params": {
							"module": "/usr/local/lib/pkcs11.so",
							"slot": "0"
						}
					}
				}
			}
		}
	}`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	prof := cfg.APIs["demo"].Profiles["default"]
	if prof.TLSSigner != "pkcs11" {
		t.Fatalf("unexpected tls_signer: %q", prof.TLSSigner)
	}
	if prof.TLSSignerParams["module"] != "/usr/local/lib/pkcs11.so" {
		t.Fatalf("unexpected module param: %q", prof.TLSSignerParams["module"])
	}
	if prof.TLSSignerParams["slot"] != "0" {
		t.Fatalf("unexpected slot param: %q", prof.TLSSignerParams["slot"])
	}
}

func TestLoad_CacheConfig(t *testing.T) {
	path := writeConfig(t, `{"cache": {"max_size": "500MB"}}`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cache.MaxSize != "500MB" {
		t.Errorf("unexpected max_size: %q", cfg.Cache.MaxSize)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := writeConfig(t, `{ "apis": [}`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	var parseErr *config.ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected *config.ParseError, got %T: %v", err, err)
	}
}

func TestLoad_UnknownField(t *testing.T) {
	path := writeConfig(t, `{"typo_field": "oops"}`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("expected 'config:' prefix in error, got: %v", err)
	}
}

func TestDefaultPath_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", dir)
	got := config.DefaultPath()
	want := filepath.Join(dir, "restish.json")
	if got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestDefaultPath_ContainsRestish(t *testing.T) {
	// Unset override to test the platform default.
	t.Setenv("RSH_CONFIG_DIR", "")
	got := config.DefaultPath()
	if !strings.Contains(got, "restish") {
		t.Errorf("DefaultPath() = %q, expected it to contain 'restish'", got)
	}
	if !strings.HasSuffix(got, "restish.json") {
		t.Errorf("DefaultPath() = %q, expected it to end with 'restish.json'", got)
	}
}

func TestSave_WritesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restish.json")
	if err := config.Save(path, &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {BaseURL: "https://api.example.com"},
		},
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "api.example.com") {
		t.Fatalf("expected written config, got:\n%s", string(data))
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), "restish.json.*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected temp files to be cleaned up, got: %v", matches)
	}
}

func TestSave_CreatesConfigDirWithSecurePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested")
	path := filepath.Join(dir, "restish.json")
	if err := config.Save(path, &config.Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("expected config dir permission 0700, got %04o", perm)
	}
}
