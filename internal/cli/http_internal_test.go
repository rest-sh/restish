package cli

import (
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
)

func TestParseByteSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"100MB", 100 * 1000 * 1000},
		{"64MiB", 64 * 1024 * 1024},
		{"1024", 1024},
		{"1GB", 1000 * 1000 * 1000},
		{"2TB", 2 * 1000 * 1000 * 1000 * 1000},
		{"1TiB", 1024 * 1024 * 1024 * 1024},
		{"0", 0},
	}

	for _, tc := range tests {
		got, err := parseByteSize(tc.in)
		if err != nil {
			t.Fatalf("parseByteSize(%q) unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseByteSize(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseByteSizeRejectsNegativeValuesClearly(t *testing.T) {
	for _, in := range []string{"-50MB", "-1"} {
		if _, err := parseByteSize(in); err == nil || err.Error() != "size must not be negative" {
			t.Fatalf("parseByteSize(%q) err = %v, want size must not be negative", in, err)
		}
	}
}

func TestCacheSizeStringToBytes_InvalidReturnsError(t *testing.T) {
	if _, err := cacheSizeStringToBytes("not-a-size"); err == nil {
		t.Fatal("expected invalid cache size error")
	}
}

func TestCacheMaxBytes_FromConfig(t *testing.T) {
	c := &CLI{cfg: &config.Config{Cache: config.CacheConfig{MaxSize: "2MB"}}}
	got, err := c.cacheMaxBytes()
	if err != nil {
		t.Fatalf("cacheMaxBytes() error = %v", err)
	}
	if want := int64(2 * 1000 * 1000); got != want {
		t.Fatalf("cacheMaxBytes() = %d, want %d", got, want)
	}
}
