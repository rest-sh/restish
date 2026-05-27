//go:build integration

package cli_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProfileTLSSignerPlugin(t *testing.T) {
	skipNoTLSSignerPlugin(t)

	pluginsParent, _ := installSharedPlugin(t, "tls-signer", testTLSSignerPluginBin, "restish-test-tls-signer")
	t.Setenv("RSH_CONFIG_DIR", pluginsParent)
	t.Setenv("PATH", "")

	caPEM, caKeyPEM, caCert := cliSelfSignedCert(t, "Test CA")
	serverCertPEM, serverKeyPEM := cliSignedCert(t, caCert, caKeyPEM, "localhost", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCertPEM, clientKeyPEM := cliSignedCert(t, caCert, caKeyPEM, "client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	certDir := t.TempDir()
	caPath := filepath.Join(certDir, "ca.pem")
	clientCertPath := filepath.Join(certDir, "client.pem")
	clientKeyPath := filepath.Join(certDir, "client.key")
	shutdownFile := filepath.Join(certDir, "shutdown.txt")
	for path, data := range map[string][]byte{
		caPath:         caPEM,
		clientCertPath: clientCertPEM,
		clientKeyPath:  clientKeyPEM,
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	t.Setenv("RSH_TLS_SIGNER_MODE", "")
	t.Setenv("RSH_TLS_SIGNER_SHUTDOWN_FILE", shutdownFile)

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("server key pair: %v", err)
	}
	clientCAPool := x509.NewCertPool()
	clientCAPool.AppendCertsFromPEM(caPEM)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ok":true}`)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAPool,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	baseURL := strings.Replace(srv.URL, "https://127.0.0.1", "https://localhost", 1)

	cfgPath := filepath.Join(pluginsParent, "restish.json")
	cfg := fmt.Sprintf(`{
		"apis": {
			"myapi": {
				"base_url": %q,
				"profiles": {
					"default": {
						"tls_signer": "test-tls-signer",
						"tls_signer_params": {
							"cert_path": %q,
							"key_path": %q
						}
					}
				}
			}
		}
	}`, baseURL, clientCertPath, clientKeyPath)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgPath
	if err := c.Run([]string{"restish", "get", "--rsh-ca-cert", caPath, "myapi"}); err != nil {
		t.Fatalf("restish get: %v", err)
	}
	if got := out.String(); got == "" || got == "null\n" {
		t.Fatalf("expected response output, got %q", got)
	}
	if _, err := os.Stat(shutdownFile); err != nil {
		t.Fatalf("expected tls signer shutdown marker, got %v", err)
	}
}

func cliSelfSignedCert(t *testing.T, commonName string) ([]byte, []byte, *x509.Certificate) {
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
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
		cert
}

func cliSignedCert(t *testing.T, caCert *x509.Certificate, caKeyPEM []byte, commonName string, usages []x509.ExtKeyUsage) ([]byte, []byte) {
	t.Helper()
	caBlock, _ := pem.Decode(caKeyPEM)
	if caBlock == nil {
		t.Fatal("invalid CA key")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caBlock.Bytes)
	if err != nil {
		t.Fatalf("parse CA key: %v", err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		t.Fatalf("serial: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           usages,
		DNSNames:              []string{"localhost"},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}
