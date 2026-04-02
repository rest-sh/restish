package request_test

import (
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/request"
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
