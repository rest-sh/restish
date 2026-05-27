package plugin

import (
	"crypto"
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	pluginwire "github.com/rest-sh/restish/v2/plugin"
)

func TestTLSSignerRejectsOversizedCertificate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	path := writeTLSSignerHelper(t, "oversized-cert")

	_, err := TLSCertificateFromPlugin(path, nil)
	if err == nil {
		t.Fatal("expected oversized certificate to fail")
	}
	if !strings.Contains(err.Error(), "certificate exceeded") {
		t.Fatalf("expected certificate cap error, got %v", err)
	}
}

func TestTLSSignerRejectsOversizedSignature(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script tests not supported on Windows")
	}
	path := writeTLSSignerHelper(t, "oversized-signature")

	cert, err := TLSCertificateFromPlugin(path, nil)
	if err != nil {
		t.Fatalf("TLSCertificateFromPlugin: %v", err)
	}
	signer, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		t.Fatalf("PrivateKey = %T, want crypto.Signer", cert.PrivateKey)
	}

	_, err = signer.Sign(crand.Reader, make([]byte, 32), crypto.SHA256)
	if err == nil {
		t.Fatal("expected oversized signature to fail")
	}
	if !strings.Contains(err.Error(), "signature exceeded") {
		t.Fatalf("expected signature cap error, got %v", err)
	}
}

func writeTLSSignerHelper(t *testing.T, mode string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "restish-tls-helper")
	script := fmt.Sprintf("#!/bin/sh\nRESTISH_TLS_SIGNER_HELPER=%s exec %s -test.run=TestTLSSignerHelperProcess --\n", strconv.Quote(mode), strconv.Quote(os.Args[0]))
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	return path
}

func TestTLSSignerHelperProcess(t *testing.T) {
	mode := os.Getenv("RESTISH_TLS_SIGNER_HELPER")
	if mode == "" {
		return
	}
	if err := runTLSSignerHelper(mode); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func runTLSSignerHelper(mode string) error {
	var init pluginwire.TLSSignerInitMsg
	if err := pluginwire.ReadMessage(os.Stdin, &init); err != nil {
		return err
	}

	switch mode {
	case "oversized-cert":
		return pluginwire.WriteMessage(os.Stdout, pluginwire.TLSSignerReadyMsg{
			Type:        pluginwire.MsgTypeTLSSignerReady,
			Certificate: make([]byte, maxTLSSignerCertificateBytes+1),
		})
	case "oversized-signature":
		der, err := testTLSSignerCertificateDER()
		if err != nil {
			return err
		}
		if err := pluginwire.WriteMessage(os.Stdout, pluginwire.TLSSignerReadyMsg{
			Type:        pluginwire.MsgTypeTLSSignerReady,
			Certificate: der,
		}); err != nil {
			return err
		}
		var sign pluginwire.TLSSignerSignMsg
		if err := pluginwire.ReadMessage(os.Stdin, &sign); err != nil {
			return err
		}
		return pluginwire.WriteMessage(os.Stdout, pluginwire.TLSSignerSignedMsg{
			Signature: make([]byte, maxTLSSignerSignatureBytes+1),
		})
	default:
		return fmt.Errorf("unknown helper mode %q", mode)
	}
}

func testTLSSignerCertificateDER() ([]byte, error) {
	key, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "restish tls signer test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	return x509.CreateCertificate(crand.Reader, template, template, &key.PublicKey, key)
}
