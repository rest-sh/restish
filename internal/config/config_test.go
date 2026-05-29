package config_test

import (
	"errors"
	"fmt"
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
	} else {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
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

func TestSavePreservesExistingConfigDirMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "restish.json")
	if err := config.Save(path, &config.Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("dir mode = %#o, want 0755", got)
	}
}

func TestSaveCreatesMissingConfigDirSecurely(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	path := filepath.Join(dir, "restish.json")
	if err := config.Save(path, &config.Config{}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir mode = %#o, want 0700", got)
	}
}

func TestLoadExplicit_MissingFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restish.json")
	_, err := config.LoadExplicit(path)
	if err == nil {
		t.Fatal("expected missing explicit config to error")
	}
	if !strings.Contains(err.Error(), "--rsh-config") ||
		!strings.Contains(err.Error(), "v2 does not fall back to the default config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_AuthRefRequiresKnownProfile(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {
				Profiles: map[string]*config.ProfileConfig{
					"default": {AuthRef: "missing"},
				},
			},
		},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected unknown auth_ref error")
	}
	if !strings.Contains(err.Error(), "auth_profiles is not defined") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_AuthAndAuthRefAreMutuallyExclusive(t *testing.T) {
	cfg := &config.Config{
		AuthProfiles: map[string]*config.AuthConfig{
			"shared": {Type: "http-basic"},
		},
		APIs: map[string]*config.APIConfig{
			"myapi": {
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						AuthRef: "shared",
						Auth:    &config.AuthConfig{Type: "http-basic"},
					},
				},
			},
		},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_CredentialAuthRefRequiresKnownProfile(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Credentials: map[string]*config.CredentialConfig{
							"UserOAuth": {AuthRef: "missing"},
						},
					},
				},
			},
		},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected unknown credential auth_ref error")
	}
	if !strings.Contains(err.Error(), "auth_profiles is not defined") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_CredentialAuthAndAuthRefAreMutuallyExclusive(t *testing.T) {
	cfg := &config.Config{
		AuthProfiles: map[string]*config.AuthConfig{
			"shared": {Type: "http-basic"},
		},
		APIs: map[string]*config.APIConfig{
			"myapi": {
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Credentials: map[string]*config.CredentialConfig{
							"UserOAuth": {
								AuthRef: "shared",
								Auth:    &config.AuthConfig{Type: "http-basic"},
							},
						},
					},
				},
			},
		},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_CredentialSatisfiesRejectsEmptyValues(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"myapi": {
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Credentials: map[string]*config.CredentialConfig{
							"UserOAuth": {Satisfies: []string{"items:read", " "}},
						},
					},
				},
			},
		},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected empty satisfies value error")
	}
	if !strings.Contains(err.Error(), "satisfies") {
		t.Fatalf("unexpected error: %v", err)
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

func TestValidateAPIName(t *testing.T) {
	valid := []string{"café", "привет", "東京", "mañana-api", "api_123"}
	for _, name := range valid {
		t.Run("valid_"+name, func(t *testing.T) {
			if err := config.ValidateAPIName(name); err != nil {
				t.Fatalf("ValidateAPIName(%q): %v", name, err)
			}
		})
	}

	invalid := []string{"", "foo/bar", "foo:bar", "foo bar", "foo?bar", "foo#bar", "foo=bar", "-foo", "_foo", "foo$bar"}
	for _, name := range invalid {
		t.Run("invalid_"+name, func(t *testing.T) {
			if err := config.ValidateAPIName(name); err == nil {
				t.Fatalf("ValidateAPIName(%q) succeeded, want error", name)
			}
		})
	}
}

func TestValidateRejectsInvalidConfiguredAPIName(t *testing.T) {
	cfg := &config.Config{
		APIs: map[string]*config.APIConfig{
			"foo/bar": {BaseURL: "https://api.example.com"},
		},
	}
	err := config.Validate(cfg)
	if err == nil {
		t.Fatal("expected invalid API name error")
	}
	if !strings.Contains(err.Error(), "API name") {
		t.Fatalf("unexpected error: %v", err)
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

func TestLoad_UnmarshalTypeErrorIncludesLineColumn(t *testing.T) {
	path := writeConfig(t, `{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "spec_files": "spec.yaml"
    }
  }
}`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for wrong field type, got nil")
	}
	var parseErr *config.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("expected *config.ParseError, got %T: %v", err, err)
	}
	if parseErr.Line != 5 || parseErr.Column == 0 {
		t.Fatalf("expected line 5/column for type error, got %d:%d", parseErr.Line, parseErr.Column)
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

func TestDiagnoseConfig_UnknownNestedLegacyField(t *testing.T) {
	path := writeConfig(t, `{
  "apis": {
    "example": {
      "base": "https://api.example.com"
    }
  }
}`)
	diags, err := config.DiagnoseConfig(path)
	if err != nil {
		t.Fatalf("DiagnoseConfig: %v", err)
	}
	if len(diags.UnknownFields) != 1 {
		t.Fatalf("UnknownFields = %#v, want one", diags.UnknownFields)
	}
	diag := diags.UnknownFields[0]
	if diag.Path != "apis.example.base" {
		t.Fatalf("Path = %q, want apis.example.base", diag.Path)
	}
	if diag.Line != 4 || diag.Column == 0 {
		t.Fatalf("expected line/column for unknown field, got %d:%d", diag.Line, diag.Column)
	}
	if !strings.Contains(diag.Hint, `v1 used "base"`) {
		t.Fatalf("expected v1 hint, got %q", diag.Hint)
	}

	_, err = config.Load(path)
	if err == nil {
		t.Fatal("expected strict Load to reject v1 field")
	}
	for _, want := range []string{"apis.example.base", `v2 uses "base_url"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected strict error to contain %q, got %v", want, err)
		}
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

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
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
		"// Original v1 files were copied to " + legacyDir + ".bak.v1.",
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
	for _, legacyPath := range []string{
		filepath.Join(legacyDir, "apis.json"),
		filepath.Join(legacyDir, "config.json"),
	} {
		if _, err := os.Stat(legacyPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected legacy file %s to be removed, got %v", legacyPath, err)
		}
	}
}

func TestLoad_MigratesLegacyFileCert(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, ".config", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  "$schema": "https://rest.sh/schemas/apis.json",
  "myapi": {
    "base": "https://api.example.com",
    "tls": {
      "cert": "/home/user/.pki/cert.pem",
      "key": "/home/user/.pki/key.pem"
    }
  }
}`)

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migration == nil {
		t.Fatal("expected migration info")
	}

	api := cfg.APIs["myapi"]
	if api == nil {
		t.Fatal("expected migrated API config")
	}
	prof := api.Profiles["default"]
	if prof == nil {
		t.Fatal("expected default profile")
	}
	if prof.ClientCertPath != "/home/user/.pki/cert.pem" {
		t.Fatalf("ClientCertPath = %q, want %q", prof.ClientCertPath, "/home/user/.pki/cert.pem")
	}
	if prof.ClientKeyPath != "/home/user/.pki/key.pem" {
		t.Fatalf("ClientKeyPath = %q, want %q", prof.ClientKeyPath, "/home/user/.pki/key.pem")
	}
}

func TestLoad_MigrationReusesMatchingExistingBackupDir(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, ".config", "restish")
	legacyAPIs := `{
  "example": { "base": "https://api.example.com" }
}`
	writeFile(t, filepath.Join(legacyDir, "apis.json"), legacyAPIs)
	writeFile(t, filepath.Join(legacyDir+".bak.v1", "apis.json"), legacyAPIs)

	cfg, err := config.Load(filepath.Join(legacyDir, "restish.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migration == nil || cfg.Migration.BackupPath != legacyDir+".bak.v1" {
		t.Fatalf("Migration = %#v, want existing backup path", cfg.Migration)
	}
	if _, err := os.Stat(filepath.Join(legacyDir, "apis.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected legacy apis.json to be removed after recovery, got %v", err)
	}
}

func TestLoad_MigrationUsesNumberedBackupWhenExistingDiffers(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, ".config", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  "example": { "base": "https://api.example.com" }
}`)
	writeFile(t, filepath.Join(legacyDir+".bak.v1", "apis.json"), `{
  "old": { "base": "https://old.example.com" }
}`)

	cfg, err := config.Load(filepath.Join(legacyDir, "restish.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migration == nil || cfg.Migration.BackupPath != legacyDir+".bak.v1.2" {
		t.Fatalf("Migration = %#v, want numbered backup path", cfg.Migration)
	}
	if _, err := os.Stat(filepath.Join(legacyDir+".bak.v1.2", "apis.json")); err != nil {
		t.Fatalf("expected numbered backup apis.json: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(legacyDir+".bak.v1.2", "*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no temp backup files left behind, got %v", matches)
	}
}

func TestLoadValidatesOperationBasePath(t *testing.T) {
	path := writeConfig(t, `{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "operation_base": "/v1"
    }
  }
}`)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("expected absolute path operation_base to load: %v", err)
	}
	if got := cfg.APIs["example"].OperationBase; got != "/v1" {
		t.Fatalf("OperationBase = %q, want /v1", got)
	}

	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "relative", raw: `"v1"`, want: "absolute path"},
		{name: "url", raw: `"https://api.example.com/v1"`, want: "absolute path"},
		{name: "query", raw: `"/v1?debug=true"`, want: "query or fragment"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, fmt.Sprintf(`{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "operation_base": %s
    }
  }
}`, tc.raw))
			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected invalid operation_base to be rejected")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}

	for _, tc := range []struct {
		name    string
		baseURL string
		want    string
	}{
		{name: "empty", baseURL: "", want: "absolute http/https URL"},
		{name: "malformed", baseURL: "://bad", want: "absolute http/https URL"},
		{name: "non-http", baseURL: "file:///tmp/api", want: "http or https"},
	} {
		t.Run("base_url_"+tc.name, func(t *testing.T) {
			path := writeConfig(t, fmt.Sprintf(`{
  "apis": {
    "example": {
      "base_url": %q,
      "operation_base": "/v1"
    }
  }
}`, tc.baseURL))
			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected invalid operation_base/base_url combination to be rejected")
			}
			if !strings.Contains(err.Error(), "apis.example.base_url") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected base_url error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestAllowedOperationOriginsValidationAndMatching(t *testing.T) {
	path := writeConfig(t, `{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "allowed_operation_origins": ["https://inference.do-ai.run", "https://*.trusted.example.com"]
    }
  }
}`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected allowed_operation_origins to load: %v", err)
	}

	patterns := []string{"https://inference.do-ai.run", "https://*.trusted.example.com"}
	for _, origin := range []string{
		"https://inference.do-ai.run/v1",
		"https://one.trusted.example.com",
		"https://deep.one.trusted.example.com",
	} {
		if !config.OperationOriginAllowed(origin, patterns) {
			t.Fatalf("expected %s to match %v", origin, patterns)
		}
	}
	for _, origin := range []string{
		"http://inference.do-ai.run",
		"https://trusted.example.com",
		"https://evil-trusted.example.com",
	} {
		if config.OperationOriginAllowed(origin, patterns) {
			t.Fatalf("expected %s not to match %v", origin, patterns)
		}
	}

	for _, raw := range []string{`"*.example.com"`, `"https://*.com"`, `"https://*bad.example.com"`, `"https://api.example.com/path"`} {
		path := writeConfig(t, fmt.Sprintf(`{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "allowed_operation_origins": [%s]
    }
  }
}`, raw))
		if _, err := config.Load(path); err == nil {
			t.Fatalf("expected invalid allowed_operation_origins entry %s", raw)
		}
	}
}

func TestURLOverridesValidationAndRewrite(t *testing.T) {
	path := writeConfig(t, `{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "url_overrides": {
        "https://api.example.com/v1": "http://localhost:8080/root",
        "https://api.example.com/": "https://fallback.example.com/"
      },
      "profiles": {
        "local": {
          "url_overrides": {
            "https://upload.example.com/": "http://localhost:9090/"
          }
        }
      }
    }
  }
}`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected url_overrides to load: %v", err)
	}

	overrides := map[string]string{
		"https://api.example.com/v1": "http://localhost:8080/root",
		"https://api.example.com/":   "https://fallback.example.com/",
	}
	got, matched, err := config.ApplyURLOverrides("https://api.example.com/v1/items?limit=1", overrides)
	if err != nil {
		t.Fatalf("ApplyURLOverrides: %v", err)
	}
	if !matched || got != "http://localhost:8080/root/items?limit=1" {
		t.Fatalf("rewrite = %q, %v; want localhost v1 rewrite", got, matched)
	}
	got, matched, err = config.ApplyURLOverrides("https://api.example.com/v10/items", overrides)
	if err != nil {
		t.Fatalf("ApplyURLOverrides v10: %v", err)
	}
	if !matched || got != "https://fallback.example.com/v10/items" {
		t.Fatalf("rewrite = %q, %v; want fallback root rewrite", got, matched)
	}
	got, matched, err = config.ApplyURLOverrides("https://api.example.com/v1/items/a%2Fb?ref=x%2Fy", overrides)
	if err != nil {
		t.Fatalf("ApplyURLOverrides escaped path: %v", err)
	}
	if !matched || got != "http://localhost:8080/root/items/a%2Fb?ref=x%2Fy" {
		t.Fatalf("rewrite = %q, %v; want escaped path preserved", got, matched)
	}
	got, matched, err = config.ApplyURLOverrides("https://api.example.com/v1/a%2Fb", map[string]string{
		"https://api.example.com/v1": "http://localhost:8080/root%2Fprefix",
	})
	if err != nil {
		t.Fatalf("ApplyURLOverrides escaped destination: %v", err)
	}
	if !matched || got != "http://localhost:8080/root%2Fprefix/a%2Fb" {
		t.Fatalf("rewrite = %q, %v; want escaped destination path preserved", got, matched)
	}

	for _, raw := range []string{`"api.example.com/"`, `"ftp://api.example.com/"`, `"https://api.example.com/?x=1"`} {
		path := writeConfig(t, fmt.Sprintf(`{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "url_overrides": {%s: "https://override.example.com/"}
    }
  }
}`, raw))
		if _, err := config.Load(path); err == nil {
			t.Fatalf("expected invalid url_overrides entry %s", raw)
		}
	}
}

func TestLoadValidatesCommandLayout(t *testing.T) {
	for _, layout := range []string{"", "flat", "tags"} {
		t.Run("valid_"+layout, func(t *testing.T) {
			path := writeConfig(t, fmt.Sprintf(`{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "command_layout": %q
    }
  }
}`, layout))
			if _, err := config.Load(path); err != nil {
				t.Fatalf("expected command_layout %q to load: %v", layout, err)
			}
		})
	}

	path := writeConfig(t, `{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "command_layout": "auto"
    }
  }
}`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected command_layout auto to be rejected")
	}
	if !strings.Contains(err.Error(), `command_layout`) || !strings.Contains(err.Error(), `"flat" or "tags"`) {
		t.Fatalf("expected command_layout error, got %v", err)
	}
}

func TestLoadValidatesRetryMaxWait(t *testing.T) {
	path := writeConfig(t, `{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "retry_max_wait": "250ms"
    }
  }
}`)
	if _, err := config.Load(path); err != nil {
		t.Fatalf("expected valid retry_max_wait to load: %v", err)
	}

	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "not duration", raw: `"not-a-duration"`, want: "invalid duration"},
		{name: "zero", raw: `"0s"`, want: "greater than 0"},
		{name: "negative", raw: `"-1s"`, want: "greater than 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, fmt.Sprintf(`{
  "apis": {
    "example": {
      "base_url": "https://api.example.com",
      "retry_max_wait": %s
    }
  }
}`, tc.raw))
			_, err := config.Load(path)
			if err == nil {
				t.Fatal("expected invalid retry_max_wait to be rejected")
			}
			if !strings.Contains(err.Error(), "apis.example.retry_max_wait") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected retry_max_wait error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestLoad_MigratesLegacyFullURLOperationBaseWithWarning(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	legacyDir := filepath.Join(home, ".config", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  "bad": {
    "base": "https://api.example.com/root",
    "operation_base": "https://other.example.com/v1"
  },
  "good": {
    "base": "https://api.example.com/root",
    "operation_base": "/v2"
  }
}`)

	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.APIs["bad"].OperationBase; got != "" {
		t.Fatalf("bad OperationBase = %q, want dropped", got)
	}
	if got := cfg.APIs["good"].OperationBase; got != "/v2" {
		t.Fatalf("good OperationBase = %q, want /v2", got)
	}
	if cfg.Migration == nil || len(cfg.Migration.Warnings) != 1 {
		t.Fatalf("expected one migration warning, got %#v", cfg.Migration)
	}
	warning := cfg.Migration.Warnings[0]
	for _, want := range []string{`api "bad"`, `https://other.example.com/v1`, "dropped invalid legacy operation_base"} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning missing %q: %q", want, warning)
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

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
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

func TestLoad_MigrationRerunAfterDeletedV2DoesNotReuseRemovedLegacy(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, ".config", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  "demo": { "base": "https://demo.example.com" }
}`)

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path := config.DefaultPath()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if cfg.Migration == nil || cfg.APIs["demo"] == nil {
		t.Fatalf("expected first load to migrate demo, got cfg=%#v", cfg)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove migrated restish.json: %v", err)
	}

	cfg, err = config.Load(path)
	if err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if cfg.Migration != nil {
		t.Fatalf("expected no second migration after legacy cleanup, got %#v", cfg.Migration)
	}
	if cfg.APIs["demo"] != nil {
		t.Fatalf("expected empty config after deleting v2 file because legacy files were cleaned up")
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

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
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

func TestLoad_RSHConfigDirSkipsLegacyMigration(t *testing.T) {
	home := t.TempDir()
	setLegacyConfigEnv(t, home)
	legacyDir := filepath.Join(home, ".config", "restish")
	writeFile(t, filepath.Join(legacyDir, "apis.json"), `{
  "legacy": {
    "base": "https://legacy.example.com"
  }
}`)

	overrideDir := filepath.Join(home, "isolated")
	t.Setenv("RSH_CONFIG_DIR", overrideDir)
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migration != nil {
		t.Fatalf("expected no migration for explicit RSH_CONFIG_DIR, got %+v", cfg.Migration)
	}
	if len(cfg.APIs) != 0 {
		t.Fatalf("expected empty config for isolated override, got %#v", cfg.APIs)
	}
	if _, err := os.Stat(filepath.Join(legacyDir+".bak.v1", "apis.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected legacy backup not to be created, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(overrideDir, "restish.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected explicit override load not to create config file, got %v", err)
	}
}
