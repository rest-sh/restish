package request_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/request"
)

func TestTLSVersionFromString(t *testing.T) {
	tests := []struct {
		in   string
		want uint16
		ok   bool
	}{
		{"", 0, true},
		{"TLS1.2", tls.VersionTLS12, true},
		{"tls1.3", tls.VersionTLS13, true},
		{"TLS12", tls.VersionTLS12, true},
		{"SSL3", 0, false},
	}

	for _, tt := range tests {
		got, err := request.TLSVersionFromString(tt.in)
		if tt.ok && err != nil {
			t.Fatalf("%q: unexpected error: %v", tt.in, err)
		}
		if !tt.ok && err == nil {
			t.Fatalf("%q: expected error", tt.in)
		}
		if got != tt.want {
			t.Fatalf("%q: got %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestTLSConfigFromOptionsWithCACert(t *testing.T) {
	caPEM, _, _ := selfSignedCert(t, "Test CA")
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, caPEM, 0o644); err != nil {
		t.Fatalf("write ca pem: %v", err)
	}

	cfg, err := request.TLSConfigFromOptions(request.Options{
		CACertPath:    caPath,
		TLSMinVersion: tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatal("expected RootCAs to be configured")
	}
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("got min version %v", cfg.MinVersion)
	}
}

func TestTLSConfigFromOptionsRequiresBothClientFiles(t *testing.T) {
	_, err := request.TLSConfigFromOptions(request.Options{
		ClientCertPath: "/tmp/cert.pem",
	})
	if err == nil {
		t.Fatal("expected error when client key is missing")
	}
}

func TestTLSConfigFromOptionsWithClientCertificate(t *testing.T) {
	certPEM, keyPEM, _ := selfSignedCert(t, "client")
	dir := t.TempDir()
	certPath := filepath.Join(dir, "client.pem")
	keyPath := filepath.Join(dir, "client.key")
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatalf("write cert pem: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key pem: %v", err)
	}

	cfg, err := request.TLSConfigFromOptions(request.Options{
		ClientCertPath: certPath,
		ClientKeyPath:  keyPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected one client certificate, got %d", len(cfg.Certificates))
	}
}

func selfSignedCert(t *testing.T, commonName string) ([]byte, []byte, *x509.Certificate) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Restish Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM, cert
}
