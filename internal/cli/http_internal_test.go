package cli

import "testing"

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
