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
