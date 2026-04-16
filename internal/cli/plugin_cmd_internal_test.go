package cli

import (
	"bytes"
	"testing"
)

func TestCappedBufferLimitsStoredBytes(t *testing.T) {
	buf := &cappedBuffer{limit: 4}
	n, err := buf.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 6 {
		t.Fatalf("Write count = %d, want 6", n)
	}
	if got := buf.Bytes(); !bytes.Equal(got, []byte("abcd")) {
		t.Fatalf("stored bytes = %q, want %q", got, []byte("abcd"))
	}
	if !buf.Truncated() {
		t.Fatal("expected buffer to report truncation")
	}
}
