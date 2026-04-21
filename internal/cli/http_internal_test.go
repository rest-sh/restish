package cli

import (
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/config"
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

func TestCacheSizeStringToBytes_InvalidReturnsZero(t *testing.T) {
	if got := cacheSizeStringToBytes("not-a-size"); got != 0 {
		t.Fatalf("expected 0 for invalid size, got %d", got)
	}
}

func TestCacheMaxBytes_FromConfig(t *testing.T) {
	c := &CLI{cfg: &config.Config{Cache: config.CacheConfig{MaxSize: "2MB"}}}
	if got, want := c.cacheMaxBytes(), int64(2*1000*1000); got != want {
		t.Fatalf("cacheMaxBytes() = %d, want %d", got, want)
	}
}
