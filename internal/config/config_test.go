package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func setLegacyConfigEnv(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	}
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
	if parseErr.Line == 0 || parseErr.Column == 0 {
		t.Fatalf("expected line:column in parse error, got %d:%d", parseErr.Line, parseErr.Column)
	}
}

func TestLoad_UnknownField(t *testing.T) {
	path := writeConfig(t, `{"apiss": {}}`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("expected 'config:' prefix in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "did you mean") {
		t.Errorf("expected unknown-field suggestion, got: %v", err)
	}
}

func TestLoad_RejectsTrailingTokens(t *testing.T) {
	path := writeConfig(t, `{"apis": {}} true`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for trailing tokens, got nil")
	}
	var parseErr *config.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *config.ParseError, got %T: %v", err, err)
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

func TestSave_ConcurrentWritesLeaveValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restish.json")
	if err := config.Save(path, &config.Config{}); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := &config.Config{
				APIs: map[string]*config.APIConfig{
					"myapi": {BaseURL: "https://api.example.com"},
				},
			}
			if i%2 == 0 {
				cfg.Cache.MaxSize = "2MB"
			}
			_ = config.Save(path, cfg)
		}(i)
	}
	wg.Wait()

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after concurrent writes: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil config after concurrent writes")
	}
}

func TestSave_IgnoresStaleCrashTempFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restish.json")
	if err := config.Save(path, &config.Config{}); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	staleTmp := path + ".stale.tmp"
	if err := os.WriteFile(staleTmp, []byte(`{"apis": {`), 0o600); err != nil {
		t.Fatalf("write stale tmp: %v", err)
	}

	want := &config.Config{APIs: map[string]*config.APIConfig{"myapi": {BaseURL: "https://api.example.com"}}}
	if err := config.Save(path, want); err != nil {
		t.Fatalf("Save with stale tmp present: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.APIs["myapi"] == nil || loaded.APIs["myapi"].BaseURL != "https://api.example.com" {
		t.Fatalf("unexpected loaded config after stale tmp save: %#v", loaded)
	}
}

func TestConfigFileHasInsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits not authoritative on Windows")
	}

	path := writeConfig(t, `{}`)
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	insecure, err := config.ConfigFileHasInsecurePermissions(path)
	if err != nil {
		t.Fatalf("ConfigFileHasInsecurePermissions: %v", err)
	}
	if !insecure {
		t.Fatalf("expected insecure permissions for 0644")
	}
}

func TestConfigFileHasInsecurePermissions_Private(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits not authoritative on Windows")
	}

	path := writeConfig(t, `{}`)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	insecure, err := config.ConfigFileHasInsecurePermissions(path)
	if err != nil {
		t.Fatalf("ConfigFileHasInsecurePermissions: %v", err)
	}
	if insecure {
		t.Fatalf("expected secure permissions for 0600")
	}
}

func TestLoad_MigratesLegacyMacOSConfig(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, "Library", "Application Support", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  // API comments stay in the v1 backup, not in migrated comments.
  "$schema": "https://rest.sh/schemas/apis.json",
  "example": {
    "base": "https://api.example.com",
    "operation_base": "/v1",
    "spec_files": ["spec.yaml"],
    "tls": {
      "pkcs11": {
        "path": "/usr/local/lib/pkcs11.so",
        "label": "device-cert"
      }
    },
    "profiles": {
      "default": {
        "headers": {
          "Accept": "application/json"
        },
        "query": {
          "verbose": "true"
        },
        "auth": {
          "name": "oauth-authorization-code",
          "params": {
            "client_id": "abc",
            "client_secret": "super-secret"
          }
        }
      }
    }
  }
}`)
	writeFile(t, filepath.Join(legacyDir, "config.json"), `{
  // v1 global config stays in the v1 backup.
  "rsh-profile": "prod"
}`)

	t.Setenv("RSH_CONFIG_DIR", filepath.Join(home, ".config", "restish"))
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migration == nil {
		t.Fatal("expected migration info")
	}
	if cfg.Migration.SourcePath != legacyDir {
		t.Fatalf("SourcePath = %q, want %q", cfg.Migration.SourcePath, legacyDir)
	}
	if cfg.Migration.BackupPath != legacyDir+".bak.v1" {
		t.Fatalf("BackupPath = %q, want %q", cfg.Migration.BackupPath, legacyDir+".bak.v1")
	}

	api := cfg.APIs["example"]
	if api == nil {
		t.Fatal("expected migrated API config")
	}
	if api.BaseURL != "https://api.example.com" {
		t.Fatalf("BaseURL = %q", api.BaseURL)
	}
	if api.OperationBase != "/v1" {
		t.Fatalf("OperationBase = %q", api.OperationBase)
	}
	if len(api.SpecFiles) != 1 || api.SpecFiles[0] != "spec.yaml" {
		t.Fatalf("SpecFiles = %v", api.SpecFiles)
	}
	prof := api.Profiles["default"]
	if prof == nil {
		t.Fatal("expected default profile")
	}
	if len(prof.Headers) != 1 || prof.Headers[0] != "Accept: application/json" {
		t.Fatalf("Headers = %v", prof.Headers)
	}
	if len(prof.Query) != 1 || prof.Query[0] != "verbose=true" {
		t.Fatalf("Query = %v", prof.Query)
	}
	if prof.Auth == nil || prof.Auth.Type != "oauth-authorization-code" || prof.Auth.Params["client_id"] != "abc" || prof.Auth.Params["client_secret"] != "super-secret" {
		t.Fatalf("Auth = %+v", prof.Auth)
	}
	if prof.TLSSigner != "pkcs11" {
		t.Fatalf("TLSSigner = %q", prof.TLSSigner)
	}
	if prof.TLSSignerParams["path"] != "/usr/local/lib/pkcs11.so" || prof.TLSSignerParams["label"] != "device-cert" {
		t.Fatalf("TLSSignerParams = %+v", prof.TLSSignerParams)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		"// Migrated from Restish v1.",
		"// Original v1 files were copied to the .bak.v1 backup directory.",
		"// Secrets are intentionally not duplicated in comments.",
		"\"base_url\": \"https://api.example.com\"",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("migrated config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "API comments stay in the v1 backup") || strings.Contains(text, "v1 global config stays in the v1 backup") {
		t.Fatalf("migrated config should not include legacy snapshot comments:\n%s", text)
	}

	for _, backupPath := range []string{
		filepath.Join(legacyDir+".bak.v1", "apis.json"),
		filepath.Join(legacyDir+".bak.v1", "config.json"),
	} {
		if _, err := os.Stat(backupPath); err != nil {
			t.Fatalf("expected backup file %s: %v", backupPath, err)
		}
	}
}

func TestLoad_MigratesLegacyLinuxConfig(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, ".config", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  "demo": {
    "base": "https://demo.example.com",
    "profiles": {
      "prod": {
        "headers": {
          "Authorization": "Bearer token"
        }
      }
    }
  }
}`)

	t.Setenv("RSH_CONFIG_DIR", legacyDir)
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIs["demo"] == nil {
		t.Fatal("expected migrated demo API")
	}
	if got := cfg.APIs["demo"].Profiles["prod"].Headers[0]; got != "Authorization: Bearer token" {
		t.Fatalf("migrated header = %q", got)
	}
	if _, err := os.Stat(filepath.Join(legacyDir+".bak.v1", "apis.json")); err != nil {
		t.Fatalf("expected apis.json backup: %v", err)
	}
}

func TestLoad_ExistingV2ConfigWinsOverLegacyMigration(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, ".config", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  "legacy": {
    "base": "https://legacy.example.com"
  }
}`)

	t.Setenv("RSH_CONFIG_DIR", legacyDir)
	path := config.DefaultPath()
	writeFile(t, path, `{
  "apis": {
    "current": {
      "base_url": "https://current.example.com"
    }
  }
}`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migration != nil {
		t.Fatalf("expected no migration, got %+v", cfg.Migration)
	}
	if cfg.APIs["current"] == nil {
		t.Fatal("expected existing v2 config to load")
	}
	if cfg.APIs["legacy"] != nil {
		t.Fatal("did not expect legacy data to overwrite v2 config")
	}
	if _, err := os.Stat(filepath.Join(legacyDir+".bak.v1", "apis.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no backup when restish.json already exists, got %v", err)
	}
}
