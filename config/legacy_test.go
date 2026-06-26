package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const legacyAPIsJSON = `{
  "$schema": "https://rest.sh/schemas/apis.json",
  "root-api": {
    "base": "https://root-api.internal.example.com/api",
    "tls": {
      "cert": "/etc/exoscale/devel-cert.pem",
      "key":  "/etc/exoscale/devel-key.pem",
      "ca_cert": "/etc/exoscale/root-ca.pem"
    },
    "profiles": {
      "default": {
        "auth": {
          "name": "oauth-authorization-code",
          "params": {
            "authorize_url": "https://dex.internal.example.com/auth",
            "client_id":     "restish",
            "scopes":        "openid email",
            "token_url":     "https://dex.internal.example.com/token"
          }
        }
      }
    }
  },
  "root-api-yubikey": {
    "base": "https://root-api.internal.example.com/api",
    "tls": {
      "pkcs11": {
        "path":  "/usr/lib/opensc-pkcs11.so",
        "label": "firstname.lastname"
      }
    },
    "profiles": {
      "default": {
        "auth": { "name": "oauth-authorization-code" }
      }
    }
  }
}`

func TestConvertLegacyAPIFileCert(t *testing.T) {
	raw := legacyAPIsEntry(t, "root-api")
	api, warnings, err := ConvertLegacyAPI("root-api", raw)
	if err != nil {
		t.Fatalf("ConvertLegacyAPI: %v", err)
	}
	if got := api.BaseURL; got != "https://root-api.internal.example.com/api" {
		t.Errorf("BaseURL = %q", got)
	}
	prof := api.Profiles["default"]
	if prof == nil {
		t.Fatal("default profile missing")
	}
	if prof.ClientCertPath != "/etc/exoscale/devel-cert.pem" {
		t.Errorf("ClientCertPath = %q", prof.ClientCertPath)
	}
	if prof.ClientKeyPath != "/etc/exoscale/devel-key.pem" {
		t.Errorf("ClientKeyPath = %q", prof.ClientKeyPath)
	}
	if prof.CACertPath != "/etc/exoscale/root-ca.pem" {
		t.Errorf("CACertPath = %q", prof.CACertPath)
	}
	if prof.TLSSigner != "" {
		t.Errorf("TLSSigner = %q, want empty for cert/key auth", prof.TLSSigner)
	}
	if prof.Auth == nil || prof.Auth.Type != "oauth-authorization-code" {
		t.Fatalf("Auth.Type = %v, want oauth-authorization-code", prof.Auth)
	}
	if got := prof.Auth.Params["client_id"]; got != "restish" {
		t.Errorf("Auth.Params[client_id] = %q", got)
	}
	for _, w := range warnings {
		if strings.Contains(w, "PKCS") {
			t.Errorf("unexpected PKCS#11 warning for cert/key entry: %s", w)
		}
	}
}

func TestConvertLegacyAPIPKCS11(t *testing.T) {
	raw := legacyAPIsEntry(t, "root-api-yubikey")
	api, warnings, err := ConvertLegacyAPI("root-api-yubikey", raw)
	if err != nil {
		t.Fatalf("ConvertLegacyAPI: %v", err)
	}
	prof := api.Profiles["default"]
	if prof == nil {
		t.Fatal("default profile missing")
	}
	if prof.TLSSigner != "pkcs11" {
		t.Errorf("TLSSigner = %q, want pkcs11", prof.TLSSigner)
	}
	if got := prof.TLSSignerParams["path"]; got != "/usr/lib/opensc-pkcs11.so" {
		t.Errorf("TLSSignerParams[path] = %q", got)
	}
	if got := prof.TLSSignerParams["label"]; got != "firstname.lastname" {
		t.Errorf("TLSSignerParams[label] = %q", got)
	}
	if prof.ClientCertPath != "" || prof.ClientKeyPath != "" {
		t.Errorf("PKCS#11 entry should not set ClientCertPath/Key: %+v", prof)
	}
	var saw bool
	for _, w := range warnings {
		if strings.Contains(w, "restish-pkcs11") {
			saw = true
		}
	}
	if !saw {
		t.Errorf("expected restish-pkcs11 warning, got: %v", warnings)
	}
}

func TestConvertLegacyAPIInvalidJSON(t *testing.T) {
	_, _, err := ConvertLegacyAPI("bad", json.RawMessage("{not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestReadLegacyAPIs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apis.json"), []byte(legacyAPIsJSON), 0o600); err != nil {
		t.Fatalf("write apis.json: %v", err)
	}
	all, err := ReadLegacyAPIs(dir)
	if err != nil {
		t.Fatalf("ReadLegacyAPIs: %v", err)
	}
	if _, ok := all["$schema"]; ok {
		t.Error("$schema key should be skipped")
	}
	if len(all) != 2 {
		t.Errorf("got %d APIs, want 2", len(all))
	}
	if all["root-api"].BaseURL == "" {
		t.Error("root-api entry missing BaseURL")
	}
	if all["root-api-yubikey"].Profiles["default"].TLSSigner != "pkcs11" {
		t.Error("root-api-yubikey TLSSigner not migrated")
	}
}

func TestReadLegacyAPIsMissingFile(t *testing.T) {
	dir := t.TempDir()
	all, err := ReadLegacyAPIs(dir)
	if err != nil {
		t.Fatalf("ReadLegacyAPIs: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("got %d APIs, want 0", len(all))
	}
}

func TestReadLegacyAPIsBadJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apis.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write apis.json: %v", err)
	}
	if _, err := ReadLegacyAPIs(dir); err == nil {
		t.Fatal("expected error for malformed apis.json")
	}
}

func TestReadLegacyAPI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apis.json"), []byte(legacyAPIsJSON), 0o600); err != nil {
		t.Fatalf("write apis.json: %v", err)
	}
	api, err := ReadLegacyAPI(dir, "root-api")
	if err != nil {
		t.Fatalf("ReadLegacyAPI: %v", err)
	}
	if api.BaseURL != "https://root-api.internal.example.com/api" {
		t.Errorf("BaseURL = %q", api.BaseURL)
	}
	if _, err := ReadLegacyAPI(dir, "missing"); err == nil {
		t.Error("expected error for missing api name")
	}
}

const v2RestishJSON = `{
  "apis": {
    "v2-api": {
      "base_url": "https://v2.example.com/api",
      "spec_files": ["openapi.json"]
    }
  }
}`

func TestReadAPIsPrefersRestishJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "restish.json"), []byte(v2RestishJSON), 0o600); err != nil {
		t.Fatalf("write restish.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "apis.json"), []byte(legacyAPIsJSON), 0o600); err != nil {
		t.Fatalf("write apis.json: %v", err)
	}
	all, err := ReadAPIs(dir)
	if err != nil {
		t.Fatalf("ReadAPIs: %v", err)
	}
	if _, ok := all["v2-api"]; !ok {
		t.Error("v2-api from restish.json missing")
	}
	if _, ok := all["root-api"]; ok {
		t.Error("v1 apis.json entry should not appear when restish.json is present")
	}
	if got := all["v2-api"].BaseURL; got != "https://v2.example.com/api" {
		t.Errorf("BaseURL = %q", got)
	}
}

func TestReadAPIsFallsBackToLegacy(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apis.json"), []byte(legacyAPIsJSON), 0o600); err != nil {
		t.Fatalf("write apis.json: %v", err)
	}
	all, err := ReadAPIs(dir)
	if err != nil {
		t.Fatalf("ReadAPIs: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("got %d APIs, want 2", len(all))
	}
	if all["root-api"].Profiles["default"].ClientCertPath != "/etc/exoscale/devel-cert.pem" {
		t.Error("legacy cert path not migrated")
	}
}

func TestReadAPIsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	all, err := ReadAPIs(dir)
	if err != nil {
		t.Fatalf("ReadAPIs: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("got %d APIs, want 0", len(all))
	}
}

func TestReadAPIsBadRestishJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "restish.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write restish.json: %v", err)
	}
	if _, err := ReadAPIs(dir); err == nil {
		t.Fatal("expected error for malformed restish.json")
	}
}

func TestReadAPIsEmptyRestishJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "restish.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("write restish.json: %v", err)
	}
	all, err := ReadAPIs(dir)
	if err != nil {
		t.Fatalf("ReadAPIs: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("got %d APIs, want 0", len(all))
	}
}

func legacyAPIsEntry(t *testing.T, name string) json.RawMessage {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(legacyAPIsJSON), &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	value, ok := raw[name]
	if !ok {
		t.Fatalf("fixture entry %q missing", name)
	}
	return value
}
