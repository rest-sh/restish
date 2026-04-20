package request_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/request"
)

var (
	tlsSignerBuildOnce sync.Once
	tlsSignerPluginBin string
	tlsSignerBuildErr  error
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

func TestTLSConfigFromOptionsWithTLSSigner(t *testing.T) {
	pluginPath := buildTLSSignerPlugin(t)
	certPEM, keyPEM, _ := selfSignedCert(t, "client")
	dir := t.TempDir()
	certPath := filepath.Join(dir, "client.pem")
	keyPath := filepath.Join(dir, "client.key")
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	cfg, err := request.TLSConfigFromOptions(request.Options{
		TLSSignerPath: pluginPath,
		TLSSignerParams: map[string]string{
			"cert_path": certPath,
			"key_path":  keyPath,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GetClientCertificate == nil {
		t.Fatal("expected GetClientCertificate to be configured")
	}
	cert, err := cfg.GetClientCertificate(&tls.CertificateRequestInfo{})
	if err != nil {
		t.Fatalf("GetClientCertificate: %v", err)
	}
	if cert == nil || cert.Leaf == nil {
		t.Fatal("expected plugin-provided certificate")
	}
}

func TestTLSSignerHandshake(t *testing.T) {
	pluginPath := buildTLSSignerPlugin(t)
	caPEM, caKeyPEM, caCert := selfSignedCert(t, "Test CA")
	serverCertPEM, serverKeyPEM := signedCert(t, caCert, caKeyPEM, "localhost", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCertPEM, clientKeyPEM := signedCert(t, caCert, caKeyPEM, "client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	clientCertPath := filepath.Join(dir, "client.pem")
	clientKeyPath := filepath.Join(dir, "client.key")
	for path, data := range map[string][]byte{
		caPath:         caPEM,
		clientCertPath: clientCertPEM,
		clientKeyPath:  clientKeyPEM,
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("server key pair: %v", err)
	}
	clientCAPool := x509.NewCertPool()
	clientCAPool.AppendCertsFromPEM(caPEM)
	server := tlsServer(t, serverCert, clientCAPool)

	t.Setenv("RSH_TLS_SIGNER_MODE", "")

	resp, err := request.Do(context.Background(), "GET", server.URL, nil, request.Options{
		CACertPath:    caPath,
		TLSSignerPath: pluginPath,
		TLSSignerParams: map[string]string{
			"cert_path": clientCertPath,
			"key_path":  clientKeyPath,
		},
	})
	if err != nil {
		t.Fatalf("request.Do: %v", err)
	}
	resp.Body.Close()
}

func TestTLSSignerErrorResponse(t *testing.T) {
	pluginPath := buildTLSSignerPlugin(t)
	caPEM, caKeyPEM, caCert := selfSignedCert(t, "Test CA")
	serverCertPEM, serverKeyPEM := signedCert(t, caCert, caKeyPEM, "localhost", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCertPEM, clientKeyPEM := signedCert(t, caCert, caKeyPEM, "client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	clientCertPath := filepath.Join(dir, "client.pem")
	clientKeyPath := filepath.Join(dir, "client.key")
	for path, data := range map[string][]byte{
		caPath:         caPEM,
		clientCertPath: clientCertPEM,
		clientKeyPath:  clientKeyPEM,
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("server key pair: %v", err)
	}
	clientCAPool := x509.NewCertPool()
	clientCAPool.AppendCertsFromPEM(caPEM)
	server := tlsServer(t, serverCert, clientCAPool)

	t.Setenv("RSH_TLS_SIGNER_MODE", "error")

	_, err = request.Do(context.Background(), "GET", server.URL, nil, request.Options{
		CACertPath:    caPath,
		TLSSignerPath: pluginPath,
		TLSSignerParams: map[string]string{
			"cert_path": clientCertPath,
			"key_path":  clientKeyPath,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "device removed") {
		t.Fatalf("expected plugin error, got %v", err)
	}
}

func TestTLSSignerErrorIncludesStderr(t *testing.T) {
	pluginPath := buildTLSSignerPlugin(t)
	caPEM, caKeyPEM, caCert := selfSignedCert(t, "Test CA")
	serverCertPEM, serverKeyPEM := signedCert(t, caCert, caKeyPEM, "localhost", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCertPEM, clientKeyPEM := signedCert(t, caCert, caKeyPEM, "client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	clientCertPath := filepath.Join(dir, "client.pem")
	clientKeyPath := filepath.Join(dir, "client.key")
	for path, data := range map[string][]byte{
		caPath:         caPEM,
		clientCertPath: clientCertPEM,
		clientKeyPath:  clientKeyPEM,
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("server key pair: %v", err)
	}
	clientCAPool := x509.NewCertPool()
	clientCAPool.AppendCertsFromPEM(caPEM)
	server := tlsServer(t, serverCert, clientCAPool)

	t.Setenv("RSH_TLS_SIGNER_MODE", "stderr-error")

	_, err = request.Do(context.Background(), "GET", server.URL, nil, request.Options{
		CACertPath:    caPath,
		TLSSignerPath: pluginPath,
		TLSSignerParams: map[string]string{
			"cert_path": clientCertPath,
			"key_path":  clientKeyPath,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "pin incorrect") {
		t.Fatalf("expected stderr text in error, got %v", err)
	}
}

func TestTLSSignerDeath(t *testing.T) {
	pluginPath := buildTLSSignerPlugin(t)
	caPEM, caKeyPEM, caCert := selfSignedCert(t, "Test CA")
	serverCertPEM, serverKeyPEM := signedCert(t, caCert, caKeyPEM, "localhost", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCertPEM, clientKeyPEM := signedCert(t, caCert, caKeyPEM, "client", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	clientCertPath := filepath.Join(dir, "client.pem")
	clientKeyPath := filepath.Join(dir, "client.key")
	for path, data := range map[string][]byte{
		caPath:         caPEM,
		clientCertPath: clientCertPEM,
		clientKeyPath:  clientKeyPEM,
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	serverCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("server key pair: %v", err)
	}
	clientCAPool := x509.NewCertPool()
	clientCAPool.AppendCertsFromPEM(caPEM)
	server := tlsServer(t, serverCert, clientCAPool)

	t.Setenv("RSH_TLS_SIGNER_MODE", "die")

	_, err = request.Do(context.Background(), "GET", server.URL, nil, request.Options{
		CACertPath:    caPath,
		TLSSignerPath: pluginPath,
		TLSSignerParams: map[string]string{
			"cert_path": clientCertPath,
			"key_path":  clientKeyPath,
		},
	})
	if err == nil || (!strings.Contains(err.Error(), "tls-signer") && !strings.Contains(err.Error(), "EOF")) {
		t.Fatalf("expected plugin death error, got %v", err)
	}
}

func TestTLSSignerTransportCloseShutsDownPlugin(t *testing.T) {
	pluginPath := buildTLSSignerPlugin(t)
	certPEM, keyPEM, _ := selfSignedCert(t, "client")

	dir := t.TempDir()
	clientCertPath := filepath.Join(dir, "client.pem")
	clientKeyPath := filepath.Join(dir, "client.key")
	shutdownFile := filepath.Join(dir, "shutdown.txt")
	for path, data := range map[string][]byte{
		clientCertPath: certPEM,
		clientKeyPath:  keyPEM,
	} {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	t.Setenv("RSH_TLS_SIGNER_SHUTDOWN_FILE", shutdownFile)

	transport := request.BuildTransport(request.Options{
		TLSSignerPath: pluginPath,
		TLSSignerParams: map[string]string{
			"cert_path": clientCertPath,
			"key_path":  clientKeyPath,
		},
	})
	closer, ok := transport.(interface{ Close() error })
	if !ok {
		t.Fatalf("expected transport to be closable, got %T", transport)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("transport.Close: %v", err)
	}
	if _, err := os.Stat(shutdownFile); err != nil {
		t.Fatalf("expected shutdown marker file: %v", err)
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

func signedCert(t *testing.T, caCert *x509.Certificate, caKeyPEM []byte, commonName string, usages []x509.ExtKeyUsage) ([]byte, []byte) {
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
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           usages,
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func buildTLSSignerPlugin(t *testing.T) string {
	t.Helper()
	tlsSignerBuildOnce.Do(func() {
		bin := filepath.Join(os.TempDir(), "restish-test-tls-signer-request-tests")
		if runtime.GOOS == "windows" {
			bin += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", bin, "./testdata/tlssigner")
		cmd.Dir = requestTestDir(t)
		cmd.Env = append(os.Environ(), "GOCACHE=/tmp/restish-gocache")
		if out, err := cmd.CombinedOutput(); err != nil {
			tlsSignerBuildErr = fmt.Errorf("build tls signer plugin: %w\n%s", err, out)
			return
		}
		tlsSignerPluginBin = bin
	})
	if tlsSignerBuildErr != nil {
		t.Fatal(tlsSignerBuildErr)
	}
	if tlsSignerPluginBin == "" {
		t.Fatal("tls signer plugin build did not produce a binary")
	}
	return tlsSignerPluginBin
}

func requestTestDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func tlsServer(t *testing.T, cert tls.Certificate, clientCAPool *x509.CertPool) *tlsServerHandle {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAPool,
	})
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	handle := &tlsServerHandle{URL: fmt.Sprintf("https://localhost:%d", port), ln: ln}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				_, _ = c.Read(buf)
				_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"))
			}(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return handle
}

type tlsServerHandle struct {
	URL string
	ln  net.Listener
}
