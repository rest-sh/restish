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
	"golang.org/x/term"
)

func main() {
	if plugin.HandleStartupFlags(os.Stdout, plugin.Manifest{
		Name:              "pkcs11",
		Version:           "1.0.0",
		Description:       "TLS signer plugin for PKCS#11 devices like YubiKey",
		RestishAPIVersion: 2,
		Hooks:             []string{"tls-signer"},
	}, nil) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)

	var initMsg plugin.TLSSignerInitMsg
	if err := dec.ReadMessage(&initMsg); err != nil {
		fail(err)
	}
	if initMsg.Type != plugin.MsgTypeInit {
		fail(fmt.Errorf("expected init message, got %q", initMsg.Type))
	}

	cfg, err := parsePKCS11Config(initMsg.Params, envMap(), promptPIN)
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
	if err := plugin.WriteMessage(os.Stdout, plugin.TLSSignerReadyMsg{
		Type:        plugin.MsgTypeTLSSignerReady,
		Certificate: leafDER,
	}); err != nil {
		fail(err)
	}

	for {
		var msg plugin.TLSSignerSignMsg
		if err := dec.ReadMessage(&msg); err != nil {
			fail(err)
		}
		if msg.Type != plugin.MsgTypeTLSSignerSign {
			continue
		}
		if len(msg.Digest) == 0 {
			_ = plugin.WriteMessage(os.Stdout, plugin.TLSSignerSignedMsg{Error: "missing digest"})
			continue
		}
		hash := crypto.Hash(msg.Hash)
		sig, err := signer.Sign(rand.Reader, msg.Digest, buildSignerOpts(hash, msg.Padding, msg.SaltLength))
		if err != nil {
			_ = plugin.WriteMessage(os.Stdout, plugin.TLSSignerSignedMsg{Error: err.Error()})
			continue
		}
		_ = plugin.WriteMessage(os.Stdout, plugin.TLSSignerSignedMsg{Signature: sig})
	}
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
