package request

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"os"
	"strings"

	internalplugin "github.com/rest-sh/restish/v2/internal/plugin"
)

// TLSVersionFromString maps CLI values like TLS1.2 and TLS1.3 to crypto/tls constants.
func TLSVersionFromString(v string) (uint16, error) {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "", "DEFAULT":
		return 0, nil
	case "TLS1.2", "TLS12":
		return tls.VersionTLS12, nil
	case "TLS1.3", "TLS13":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unsupported TLS min version %q; expected TLS1.2 or TLS1.3", v)
	}
}

// TLSConfigFromOptions builds a TLS config for the given request options.
func TLSConfigFromOptions(opts Options) (*tls.Config, error) {
	cfg, _, err := TLSConfigWithCleanupFromOptions(opts)
	return cfg, err
}

// TLSConfigWithCleanupFromOptions builds a TLS config and returns an optional
// cleanup for plugin-backed client certificate resources.
func TLSConfigWithCleanupFromOptions(opts Options) (*tls.Config, io.Closer, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: opts.Insecure, //nolint:gosec
		MinVersion:         opts.TLSMinVersion,
	}

	if opts.TLSSignerPath != "" && (opts.ClientCertPath != "" || opts.ClientKeyPath != "") {
		return nil, nil, fmt.Errorf("tls signer cannot be used together with client certificate/key files")
	}

	if opts.ClientCertPath != "" || opts.ClientKeyPath != "" {
		if opts.ClientCertPath == "" || opts.ClientKeyPath == "" {
			return nil, nil, fmt.Errorf("both client certificate and key are required for mTLS")
		}
		cert, err := tls.LoadX509KeyPair(opts.ClientCertPath, opts.ClientKeyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("loading client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	var cleanup io.Closer
	if opts.TLSSignerPath != "" {
		cert, err := internalplugin.TLSCertificateFromPlugin(opts.TLSSignerPath, opts.TLSSignerParams)
		if err != nil {
			return nil, nil, err
		}
		if closer, ok := cert.PrivateKey.(io.Closer); ok {
			cleanup = closer
		}
		cfg.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return cert, nil
		}
	}

	if opts.CACertPath != "" {
		pool, err := bestEffortSystemCertPool()
		if err != nil {
			if cleanup != nil {
				_ = cleanup.Close()
			}
			return nil, nil, err
		}
		data, err := os.ReadFile(opts.CACertPath)
		if err != nil {
			if cleanup != nil {
				_ = cleanup.Close()
			}
			return nil, nil, fmt.Errorf("reading CA certificate: %w", err)
		}
		if !pool.AppendCertsFromPEM(data) {
			if cleanup != nil {
				_ = cleanup.Close()
			}
			return nil, nil, fmt.Errorf("parsing CA certificate %q", opts.CACertPath)
		}
		cfg.RootCAs = pool
	}

	return cfg, cleanup, nil
}

func bestEffortSystemCertPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		return x509.NewCertPool(), nil
	}
	return pool, nil
}
