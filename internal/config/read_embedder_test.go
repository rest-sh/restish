package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAllV2(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "restish.json"), []byte(`{
		"apis": {
			"billing": {
				"base_url": "https://billing.example.com",
				"profiles": {
					"default": {
						"client_cert": "/etc/cert.pem",
						"auth": { "type": "bearer", "params": { "token": "abc" } }
					}
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	apis, err := ReadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(apis) != 1 {
		t.Fatalf("expected 1 api, got %d", len(apis))
	}
	billing := apis["billing"]
	if billing == nil {
		t.Fatal("missing billing api")
	}
	if billing.BaseURL != "https://billing.example.com" {
		t.Errorf("base_url = %q", billing.BaseURL)
	}
	if billing.Profiles["default"].ClientCertPath != "/etc/cert.pem" {
		t.Errorf("client_cert = %q", billing.Profiles["default"].ClientCertPath)
	}
	if billing.Profiles["default"].Auth.Type != "bearer" {
		t.Errorf("auth.type = %q", billing.Profiles["default"].Auth.Type)
	}
}

func TestReadAllV1Fallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apis.json"), []byte(`{
		"$schema": "https://rest.sh/schemas/apis.json",
		"myapi": {
			"base": "https://api.example.com",
			"tls": {
				"cert": "/etc/cert.pem",
				"key": "/etc/key.pem"
			},
			"profiles": {
				"default": {
					"auth": { "name": "bearer", "params": { "token": "abc" } }
				}
			}
		}
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	apis, err := ReadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if apis["myapi"] == nil {
		t.Fatal("missing myapi in v1 fallback")
	}
	if apis["myapi"].BaseURL != "https://api.example.com" {
		t.Errorf("v1 base conversion: base_url = %q", apis["myapi"].BaseURL)
	}
	if apis["myapi"].Profiles["default"].ClientCertPath != "/etc/cert.pem" {
		t.Errorf("v1 cert conversion: client_cert = %q", apis["myapi"].Profiles["default"].ClientCertPath)
	}
}

func TestReadProfileMissing(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadProfile(dir, "nope"); err == nil {
		t.Fatal("expected error for missing api")
	}
}
