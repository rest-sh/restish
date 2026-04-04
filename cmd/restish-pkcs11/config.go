package main

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ThalesIgnite/crypto11"
)

var defaultPKCS11Paths = func() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/opt/homebrew/lib/opensc-pkcs11.so",
			"/usr/local/lib/opensc-pkcs11.so",
		}
	default:
		return []string{
			"/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so",
			"/usr/lib/pkcs11/opensc-pkcs11.so",
			"/usr/lib64/pkcs11/opensc-pkcs11.so",
		}
	}
}

type pkcs11Config struct {
	ModulePath        string
	TokenLabel        string
	TokenSerial       string
	SlotNumber        *int
	PIN               string
	LoginNotSupported bool
}

func parsePKCS11Config(params map[string]any, env map[string]string, promptPIN func() (string, error)) (*pkcs11Config, error) {
	modulePath := firstString(params, "module", "path")
	if modulePath == "" {
		modulePath = env["PKCS11_MODULE_PATH"]
	}
	if modulePath == "" {
		modulePath = defaultPKCS11ModulePath()
	}
	if modulePath == "" {
		return nil, fmt.Errorf("pkcs11 module path is required; set tls_signer_params.module or tls_signer_params.path")
	}

	label := firstString(params, "token_label", "label")
	serial := firstString(params, "token_serial", "serial")
	slotValue := firstString(params, "slot")
	slot, err := parseOptionalInt(slotValue)
	if err != nil {
		return nil, fmt.Errorf("invalid slot %q: %w", slotValue, err)
	}
	if countNonEmpty(label, serial)+boolCount(slot != nil) != 1 {
		return nil, fmt.Errorf("exactly one of token_label/label, token_serial/serial, or slot must be set")
	}

	pin := firstString(params, "pin")
	if pin == "" {
		pinEnvName := firstString(params, "pin_env")
		if pinEnvName == "" {
			pinEnvName = "PKCS11_PIN"
		}
		pin = env[pinEnvName]
	}
	loginNotSupported := firstBool(params, "login_not_supported")
	if !loginNotSupported && pin == "" {
		if promptPIN == nil {
			return nil, fmt.Errorf("pkcs11 pin is required; set tls_signer_params.pin or %s", "PKCS11_PIN")
		}
		pin, err = promptPIN()
		if err != nil {
			return nil, err
		}
	}

	return &pkcs11Config{
		ModulePath:        modulePath,
		TokenLabel:        label,
		TokenSerial:       serial,
		SlotNumber:        slot,
		PIN:               pin,
		LoginNotSupported: loginNotSupported,
	}, nil
}

func (c *pkcs11Config) crypto11Config() *crypto11.Config {
	return &crypto11.Config{
		Path:              c.ModulePath,
		TokenLabel:        c.TokenLabel,
		TokenSerial:       c.TokenSerial,
		SlotNumber:        c.SlotNumber,
		Pin:               c.PIN,
		LoginNotSupported: c.LoginNotSupported,
	}
}

func defaultPKCS11ModulePath() string {
	for _, path := range defaultPKCS11Paths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func buildSignerOpts(hash crypto.Hash, padding string, saltLength int) crypto.SignerOpts {
	if padding == "pss" {
		return &rsa.PSSOptions{Hash: hash, SaltLength: saltLength}
	}
	return hash
}

func firstString(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if text, ok := params[key].(string); ok && text != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func firstBool(params map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := params[key].(bool); ok {
			return value
		}
	}
	return false
}

func parseOptionalInt(value string) (*int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func countNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func boolCount(v bool) int {
	if v {
		return 1
	}
	return 0
}
