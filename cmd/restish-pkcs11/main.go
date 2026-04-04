package main

import (
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ThalesIgnite/crypto11"
	"github.com/danielgtaylor/restish/v2/plugin"
	"github.com/fxamacker/cbor/v2"
	"golang.org/x/term"
)

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--rsh-plugin-manifest" {
			writeManifest()
			return
		}
	}

	var initMsg map[string]any
	if err := plugin.ReadMessage(os.Stdin, &initMsg); err != nil {
		fail(err)
	}
	if initMsg["type"] != "init" {
		fail(fmt.Errorf("expected init message"))
	}
	params, _ := initMsg["params"].(map[string]any)
	if params == nil {
		params = map[string]any{}
	}

	cfg, err := parsePKCS11Config(params, envMap(), promptPIN)
	if err != nil {
		fail(err)
	}
	ctx, err := crypto11.Configure(cfg.crypto11Config())
	if err != nil {
		fail(err)
	}
	defer ctx.Close()

	cert, signer, err := loadCertificate(ctx)
	if err != nil {
		fail(err)
	}
	leafDER, err := certificateDER(cert)
	if err != nil {
		fail(err)
	}
	if err := plugin.WriteMessage(os.Stdout, map[string]any{
		"type":        "ready",
		"certificate": leafDER,
	}); err != nil {
		fail(err)
	}

	for {
		var msg map[string]any
		if err := plugin.ReadMessage(os.Stdin, &msg); err != nil {
			fail(err)
		}
		if msg["type"] != "sign" {
			continue
		}
		digest := msgBytes(msg["digest"])
		if len(digest) == 0 {
			_ = plugin.WriteMessage(os.Stdout, map[string]any{"error": "missing digest"})
			continue
		}
		hash := msgHash(msg["hash"])
		padding, _ := msg["padding"].(string)
		saltLength := msgInt(msg["salt_length"])
		sig, err := signer.Sign(rand.Reader, digest, buildSignerOpts(hash, padding, saltLength))
		if err != nil {
			_ = plugin.WriteMessage(os.Stdout, map[string]any{"error": err.Error()})
			continue
		}
		_ = plugin.WriteMessage(os.Stdout, map[string]any{"signature": sig})
	}
}

func writeManifest() {
	data, err := cbor.Marshal(map[string]any{
		"name":                "pkcs11",
		"version":             "1.0.0",
		"description":         "TLS signer plugin for PKCS#11 devices like YubiKey",
		"restish_api_version": 1,
		"hooks":               []string{"tls-signer"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(2)
	}
	_, _ = os.Stdout.Write(data)
}

func loadCertificate(ctx *crypto11.Context) (*tls.Certificate, crypto11.Signer, error) {
	certs, err := ctx.FindAllPairedCertificates()
	if err != nil {
		return nil, nil, err
	}
	if len(certs) == 0 {
		return nil, nil, fmt.Errorf("no certificate found in your pkcs11 device")
	}
	if len(certs) > 1 {
		return nil, nil, fmt.Errorf("got more than one certificate; narrow the token selection")
	}
	signer, ok := certs[0].PrivateKey.(crypto11.Signer)
	if !ok {
		return nil, nil, fmt.Errorf("pkcs11 private key does not implement crypto11.Signer")
	}
	return &certs[0], signer, nil
}

func certificateDER(cert *tls.Certificate) ([]byte, error) {
	if cert == nil || len(cert.Certificate) == 0 || len(cert.Certificate[0]) == 0 {
		return nil, fmt.Errorf("pkcs11 certificate is empty")
	}
	return cert.Certificate[0], nil
}

func promptPIN() (string, error) {
	ttyPath := "/dev/tty"
	if runtime.GOOS == "windows" {
		ttyPath = "CONIN$"
	}
	in, err := os.OpenFile(ttyPath, os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("pkcs11 pin is required and no tty is available")
	}
	defer in.Close()
	fmt.Fprint(in, "PIN for your PKCS#11 device: ")
	pin, err := term.ReadPassword(int(in.Fd()))
	fmt.Fprintln(in)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(pin)), nil
}

func envMap() map[string]string {
	out := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

func msgBytes(v any) []byte {
	switch data := v.(type) {
	case []byte:
		return data
	case []any:
		out := make([]byte, 0, len(data))
		for _, item := range data {
			switch n := item.(type) {
			case uint64:
				out = append(out, byte(n))
			case int64:
				out = append(out, byte(n))
			case int:
				out = append(out, byte(n))
			}
		}
		return out
	default:
		return nil
	}
}

func msgHash(v any) crypto.Hash {
	return crypto.Hash(msgInt(v))
}

func msgInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
