package cli

import (
	"net/url"
	"testing"
)

func TestCertDialAddress(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://example.com", "example.com:443"},
		{"https://example.com:8443", "example.com:8443"},
		{"https://[::1]", "[::1]:443"},
		{"https://[::1]:8443", "[::1]:8443"},
	}

	for _, tt := range tests {
		u, err := url.Parse(tt.in)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.in, err)
		}
		if got := certDialAddress(u); got != tt.want {
			t.Fatalf("certDialAddress(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeCertTargetDefaultsToHTTPS(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"example.com", "https://example.com"},
		{"localhost:8443", "https://localhost:8443"},
		{":8443", "https://localhost:8443"},
	}

	for _, tt := range tests {
		got, err := normalizeCertTarget(tt.in, "")
		if err != nil {
			t.Fatalf("normalizeCertTarget(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("normalizeCertTarget(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
