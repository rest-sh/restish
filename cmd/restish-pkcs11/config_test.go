package main

import (
	"crypto"
	"crypto/rsa"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miekg/pkcs11"
)

func TestParsePKCS11ConfigAliases(t *testing.T) {
	cfg, err := parsePKCS11Config(map[string]string{
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
	_, err := parsePKCS11Config(map[string]string{
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
	cfg, err := parsePKCS11Config(map[string]string{
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

func TestParsePKCS11ConfigAutoDetectsSingleToken(t *testing.T) {
	fake := &fakePKCS11Ctx{slots: []uint{42}}
	restore := stubPKCS11Ctx(t, fake)
	defer restore()

	cfg, err := parsePKCS11Config(map[string]string{
		"module": "/tmp/opensc.so",
		"pin":    "123456",
	}, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("parsePKCS11Config: %v", err)
	}
	if cfg.SlotNumber == nil || *cfg.SlotNumber != 42 {
		t.Fatalf("unexpected slot: %#v", cfg.SlotNumber)
	}
	if !fake.finalized || !fake.destroyed {
		t.Fatalf("expected pkcs11 context cleanup, finalized=%v destroyed=%v", fake.finalized, fake.destroyed)
	}
}

func TestParsePKCS11ConfigAutoDetectRequiresToken(t *testing.T) {
	fake := &fakePKCS11Ctx{}
	restore := stubPKCS11Ctx(t, fake)
	defer restore()

	_, err := parsePKCS11Config(map[string]string{
		"module": "/tmp/opensc.so",
		"pin":    "123456",
	}, map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), "no pkcs11 tokens found; plug in your device") {
		t.Fatalf("expected no-token error, got %v", err)
	}
	if !fake.finalized || !fake.destroyed {
		t.Fatalf("expected pkcs11 context cleanup, finalized=%v destroyed=%v", fake.finalized, fake.destroyed)
	}
}

func TestParsePKCS11ConfigAutoDetectListsMultipleTokens(t *testing.T) {
	fake := &fakePKCS11Ctx{
		slots: []uint{3, 9},
		tokenInfo: map[uint]pkcs11.TokenInfo{
			3: {Label: "YubiKey PIV   "},
			9: {Label: "Backup"},
		},
	}
	restore := stubPKCS11Ctx(t, fake)
	defer restore()

	_, err := parsePKCS11Config(map[string]string{
		"module": "/tmp/opensc.so",
		"pin":    "123456",
	}, map[string]string{}, nil)
	want := `found 2 pkcs11 tokens ("YubiKey PIV" (slot 3), "Backup" (slot 9)); set token_label/label, token_serial/serial, or slot to pick one`
	if err == nil || err.Error() != want {
		t.Fatalf("unexpected error:\nwant %q\ngot  %v", want, err)
	}
	if !fake.finalized || !fake.destroyed {
		t.Fatalf("expected pkcs11 context cleanup, finalized=%v destroyed=%v", fake.finalized, fake.destroyed)
	}
}

func TestParsePKCS11ConfigAutoDetectHandlesLoadError(t *testing.T) {
	restore := stubPKCS11Ctx(t, nil)
	defer restore()

	_, err := parsePKCS11Config(map[string]string{
		"module": "/tmp/missing-pkcs11.so",
		"pin":    "123456",
	}, map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), `could not load pkcs11 module "/tmp/missing-pkcs11.so"`) {
		t.Fatalf("expected load error, got %v", err)
	}
}

func TestParsePKCS11ConfigAutoDetectCleansUpInitializeError(t *testing.T) {
	fake := &fakePKCS11Ctx{initializeErr: errors.New("module unavailable")}
	restore := stubPKCS11Ctx(t, fake)
	defer restore()

	_, err := parsePKCS11Config(map[string]string{
		"module": "/tmp/opensc.so",
		"pin":    "123456",
	}, map[string]string{}, nil)
	if err == nil || !strings.Contains(err.Error(), "pkcs11 initialize: module unavailable") {
		t.Fatalf("expected initialize error, got %v", err)
	}
	if fake.finalized {
		t.Fatal("did not expect finalize after initialize failure")
	}
	if !fake.destroyed {
		t.Fatal("expected destroy after initialize failure")
	}
}

func TestParsePKCS11ConfigPromptsForPIN(t *testing.T) {
	prompted := false
	cfg, err := parsePKCS11Config(map[string]string{
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

type fakePKCS11Ctx struct {
	initializeErr error
	finalized     bool
	destroyed     bool
	slots         []uint
	slotErr       error
	tokenInfo     map[uint]pkcs11.TokenInfo
	tokenInfoErr  map[uint]error
}

func (f *fakePKCS11Ctx) Initialize() error {
	return f.initializeErr
}

func (f *fakePKCS11Ctx) Finalize() error {
	f.finalized = true
	return nil
}

func (f *fakePKCS11Ctx) Destroy() {
	f.destroyed = true
}

func (f *fakePKCS11Ctx) GetSlotList(tokenPresent bool) ([]uint, error) {
	return f.slots, f.slotErr
}

func (f *fakePKCS11Ctx) GetTokenInfo(slotID uint) (pkcs11.TokenInfo, error) {
	if err := f.tokenInfoErr[slotID]; err != nil {
		return pkcs11.TokenInfo{}, err
	}
	if info, ok := f.tokenInfo[slotID]; ok {
		return info, nil
	}
	return pkcs11.TokenInfo{}, nil
}

func stubPKCS11Ctx(t *testing.T, fake pkcs11TokenEnumerator) func() {
	t.Helper()
	orig := newPKCS11Ctx
	newPKCS11Ctx = func(modulePath string) pkcs11TokenEnumerator { return fake }
	return func() {
		newPKCS11Ctx = orig
	}
}
