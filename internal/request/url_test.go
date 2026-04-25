package request_test

import (
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/request"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		override string
		want     string
	}{
		{
			name: "already https",
			raw:  "https://api.example.com/items",
			want: "https://api.example.com/items",
		},
		{
			name: "no scheme adds https",
			raw:  "api.example.com/items",
			want: "https://api.example.com/items",
		},
		{
			name: "localhost defaults to http",
			raw:  "localhost:8080/items",
			want: "http://localhost:8080/items",
		},
		{
			name: "loopback defaults to http",
			raw:  "127.0.0.1:8080/items",
			want: "http://127.0.0.1:8080/items",
		},
		{
			name: "bare port with path",
			raw:  ":8080/path",
			want: "http://localhost:8080/path",
		},
		{
			name: "bare port no path",
			raw:  ":8080",
			want: "http://localhost:8080",
		},
		{
			name: "explicit http unchanged",
			raw:  "http://api.example.com",
			want: "http://api.example.com",
		},
		{
			name: "preserves path and query",
			raw:  "api.example.com/items?foo=bar",
			want: "https://api.example.com/items?foo=bar",
		},
		{
			name:     "server override replaces scheme and host",
			raw:      "https://api.example.com/items",
			override: "https://staging.example.com",
			want:     "https://staging.example.com/items",
		},
		{
			name:     "server override changes scheme",
			raw:      "https://api.example.com/items",
			override: "http://localhost:9000",
			want:     "http://localhost:9000/items",
		},
		{
			name:     "server override preserves query string",
			raw:      "https://api.example.com/items?page=2",
			override: "https://staging.example.com",
			want:     "https://staging.example.com/items?page=2",
		},
		{
			name:     "server override path prefixes request path",
			raw:      "https://api.example.com/items?page=2",
			override: "https://staging.example.com/v2",
			want:     "https://staging.example.com/v2/items?page=2",
		},
		{
			name:     "server override path handles root request",
			raw:      "https://api.example.com",
			override: "https://staging.example.com/v2",
			want:     "https://staging.example.com/v2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := request.Normalize(tc.raw, tc.override)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("Normalize(%q, %q)\n  got  %q\n  want %q", tc.raw, tc.override, got, tc.want)
			}
		})
	}
}

func TestNormalizeRejectsNonHTTPSOverrideSchemes(t *testing.T) {
	_, err := request.Normalize("https://api.example.com/items", "file:///tmp/evil")
	if err == nil {
		t.Fatal("expected invalid override scheme to fail")
	}
	if !strings.Contains(err.Error(), "http or https") {
		t.Fatalf("expected scheme validation error, got %v", err)
	}
}
