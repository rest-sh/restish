package request

import (
	"crypto/x509"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTLSConfigFromOptionsFailsClosedWhenSystemPoolFails(t *testing.T) {
	orig := systemCertPool
	t.Cleanup(func() { systemCertPool = orig })
	systemCertPool = func() (*x509.CertPool, error) {
		return nil, errors.New("trust store unavailable")
	}

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, []byte("not reached"), 0o644); err != nil {
		t.Fatalf("write ca: %v", err)
	}

	_, err := TLSConfigFromOptions(Options{CACertPath: caPath})
	if err == nil {
		t.Fatal("expected trust store error")
	}
	if !strings.Contains(err.Error(), "loading system certificate pool") ||
		!strings.Contains(err.Error(), "trust store unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
}
