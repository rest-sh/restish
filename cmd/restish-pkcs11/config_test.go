package main

import (
	"crypto"
	"crypto/rsa"
	"path/filepath"
	"testing"
)

func TestParsePKCS11ConfigAliases(t *testing.T) {
	cfg, err := parsePKCS11Config(map[string]any{
		"path":  "/tmp/opensc.so",
		"label": "YubiKey PIV",
		"pin":   "123456",
	}, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("parsePKCS11Config: %v", err)
	}
	if cfg.ModulePath != "/tmp/opensc.so" {
		t.Fatalf("unexpected module path: %q", cfg.ModulePath)
	}
	if cfg.TokenLabel != "YubiKey PIV" {
		t.Fatalf("unexpected token label: %q", cfg.TokenLabel)
	}
	if cfg.PIN != "123456" {
		t.Fatalf("unexpected pin: %q", cfg.PIN)
	}
}

func TestParsePKCS11ConfigRequiresSingleSelector(t *testing.T) {
	_, err := parsePKCS11Config(map[string]any{
		"module":      "/tmp/opensc.so",
		"token_label": "A",
		"serial":      "B",
		"pin":         "123456",
	}, map[string]string{}, nil)
	if err == nil {
		t.Fatal("expected selector error")
	}
}

func TestParsePKCS11ConfigUsesPinEnvAndSlot(t *testing.T) {
	cfg, err := parsePKCS11Config(map[string]any{
		"module":  "/tmp/opensc.so",
		"slot":    "7",
		"pin_env": "MY_PIN",
	}, map[string]string{"MY_PIN": "secret"}, nil)
	if err != nil {
		t.Fatalf("parsePKCS11Config: %v", err)
	}
	if cfg.SlotNumber == nil || *cfg.SlotNumber != 7 {
		t.Fatalf("unexpected slot: %#v", cfg.SlotNumber)
	}
	if cfg.PIN != "secret" {
		t.Fatalf("unexpected pin: %q", cfg.PIN)
	}
}

func TestParsePKCS11ConfigPromptsForPIN(t *testing.T) {
	prompted := false
	cfg, err := parsePKCS11Config(map[string]any{
		"module":      "/tmp/opensc.so",
		"token_label": "YubiKey PIV",
	}, map[string]string{}, func() (string, error) {
		prompted = true
		return "654321", nil
	})
	if err != nil {
		t.Fatalf("parsePKCS11Config: %v", err)
	}
	if !prompted {
		t.Fatal("expected PIN prompt")
	}
	if cfg.PIN != "654321" {
		t.Fatalf("unexpected pin: %q", cfg.PIN)
	}
}

func TestDefaultPKCS11ModulePathEmptyWhenNoCandidate(t *testing.T) {
	orig := defaultPKCS11Paths
	defaultPKCS11Paths = func() []string { return []string{filepath.Join(t.TempDir(), "missing.so")} }
	t.Cleanup(func() { defaultPKCS11Paths = orig })
	if got := defaultPKCS11ModulePath(); got != "" {
		t.Fatalf("expected empty path, got %q", got)
	}
}

func TestBuildSignerOpts(t *testing.T) {
	if got := buildSignerOpts(crypto.SHA256, "", 0); got.HashFunc() != crypto.SHA256 {
		t.Fatalf("unexpected hash func: %v", got.HashFunc())
	}
	pss, ok := buildSignerOpts(crypto.SHA384, "pss", rsa.PSSSaltLengthEqualsHash).(*rsa.PSSOptions)
	if !ok {
		t.Fatal("expected PSS options")
	}
	if pss.Hash != crypto.SHA384 || pss.SaltLength != rsa.PSSSaltLengthEqualsHash {
		t.Fatalf("unexpected pss opts: %#v", pss)
	}
}

func TestEnvMap(t *testing.T) {
	t.Setenv("RESTISH_PKCS11_TEST", "value")
	if got := envMap()["RESTISH_PKCS11_TEST"]; got != "value" {
		t.Fatalf("unexpected env value: %q", got)
	}
}
