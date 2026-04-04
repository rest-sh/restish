package request

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
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
	cfg := &tls.Config{
		InsecureSkipVerify: opts.Insecure, //nolint:gosec
		MinVersion:         opts.TLSMinVersion,
	}

	if opts.ClientCertPath != "" || opts.ClientKeyPath != "" {
		if opts.ClientCertPath == "" || opts.ClientKeyPath == "" {
			return nil, fmt.Errorf("both client certificate and key are required for mTLS")
		}
		cert, err := tls.LoadX509KeyPair(opts.ClientCertPath, opts.ClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	if opts.CACertPath != "" {
		pool, err := bestEffortSystemCertPool()
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(opts.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate: %w", err)
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("parsing CA certificate %q", opts.CACertPath)
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}

func bestEffortSystemCertPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		return x509.NewCertPool(), nil
	}
	return pool, nil
}
