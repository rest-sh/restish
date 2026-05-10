package cli_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/internal/cli"
)

func TestCertCommandShowsIssuerAndSubject(t *testing.T) {
	server, caPath := newTLSServerWithChain(t, time.Now().Add(30*24*time.Hour), false)
	defer server.Close()
	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "cert", "--rsh-ca-cert", caPath, server.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Subject:") {
		t.Fatalf("expected subject in output, got %q", got)
	}
	if !strings.Contains(got, "Issuer:") {
		t.Fatalf("expected issuer in output, got %q", got)
	}
}

func TestCertCommandShowsMultipleCertificates(t *testing.T) {
	server, caPath := newTLSServerWithChain(t, time.Now().Add(30*24*time.Hour), true)
	defer server.Close()

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "cert", "--rsh-ca-cert", caPath, server.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Leaf Certificate") || !strings.Contains(got, "Chain 1 Certificate") {
		t.Fatalf("expected multiple certs in output, got %q", got)
	}
}

func TestCertWarnDaysExpiresSoon(t *testing.T) {
	server, caPath := newTLSServerWithChain(t, time.Now().Add(24*time.Hour), false)
	defer server.Close()

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "cert", "--rsh-ca-cert", caPath, "--warn-days", "2", server.URL})
	var exitErr *cli.ExitCodeError
	if err == nil || !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("expected ExitCodeError{1}, got %v", err)
	}
}

func TestCertWarnDaysValidLonger(t *testing.T) {
	server, caPath := newTLSServerWithChain(t, time.Now().Add(30*24*time.Hour), false)
	defer server.Close()

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "cert", "--rsh-ca-cert", caPath, "--warn-days", "2", server.URL}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCertRejectsNonTLSScheme(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "cert", "http://example.com"})
	if err == nil {
		t.Fatal("expected non-TLS scheme error")
	}
	if !strings.Contains(err.Error(), "unsupported non-TLS scheme") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newTLSServerWithChain(t *testing.T, leafExpiry time.Time, includeIntermediate bool) (*httptest.Server, string) {
	t.Helper()

	rootCert, rootKey, rootPEM := mustCertificateAuthority(t, "Restish Root CA", time.Now().Add(365*24*time.Hour), nil, nil)
	issuerCert := rootCert
	issuerKey := rootKey
	chain := [][]byte{}

	if includeIntermediate {
		intermediateCert, intermediateKey, intermediatePEM := mustCertificateAuthority(t, "Restish Intermediate CA", time.Now().Add(180*24*time.Hour), rootCert, rootKey)
		issuerCert = intermediateCert
		issuerKey = intermediateKey
		chain = append(chain, intermediatePEM)
	}

	leafTLS := mustLeafCert(t, issuerCert, issuerKey, leafExpiry, chain)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{leafTLS}}
	server.StartTLS()

	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, rootPEM, 0o644); err != nil {
		t.Fatalf("write ca pem: %v", err)
	}
	return server, caPath
}

func mustCertificateAuthority(t *testing.T, commonName string, notAfter time.Time, parent *x509.Certificate, parentKey *rsa.PrivateKey) (*x509.Certificate, *rsa.PrivateKey, []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"Restish Test"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	if parent == nil {
		parent = template
		parentKey = key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("create ca cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse ca cert: %v", err)
	}
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func mustLeafCert(t *testing.T, issuer *x509.Certificate, issuerKey *rsa.PrivateKey, notAfter time.Time, extraChain [][]byte) tls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "127.0.0.1",
			Organization: []string{"Restish Test"},
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, issuer, &key.PublicKey, issuerKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	for _, item := range extraChain {
		certPEM = append(certPEM, item...)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("tls key pair: %v", err)
	}
	return tlsCert
}
