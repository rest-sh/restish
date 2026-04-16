package output

import (
	"bytes"
	"testing"
)

func TestIndentBlockPreservesLines(t *testing.T) {
	got := indentBlock([]byte("{\n  \"a\": 1\n}"), "  ")
	want := []byte("  {\n    \"a\": 1\n  }")
	if !bytes.Equal(got, want) {
		t.Fatalf("indentBlock() = %q, want %q", got, want)
	}
}
