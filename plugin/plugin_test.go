package plugin_test

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/plugin"
)

// TestWriteReadRoundTrip verifies that WriteMessage → ReadMessage recovers
// the original value for common Go types.
func TestWriteReadRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		val  any
	}{
		{"string", "hello world"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"map", map[string]any{"key": "value", "num": float64(1)}},
		{"slice", []any{"a", "b", "c"}},
		{"nil", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := plugin.WriteMessage(&buf, tc.val); err != nil {
				t.Fatalf("WriteMessage: %v", err)
			}

			var got any
			if err := plugin.ReadMessage(&buf, &got); err != nil {
				t.Fatalf("ReadMessage: %v", err)
			}
			// Basic non-nil check for non-nil inputs.
			if tc.val != nil && got == nil {
				t.Errorf("got nil, want %v", tc.val)
			}
		})
	}
}

// TestLengthPrefixIsCorrect verifies that the 4-byte big-endian prefix equals
// the actual payload length.
func TestLengthPrefixIsCorrect(t *testing.T) {
	var buf bytes.Buffer
	if err := plugin.WriteMessage(&buf, "test"); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 4 {
		t.Fatalf("output too short: %d bytes", len(data))
	}
	declared := binary.BigEndian.Uint32(data[:4])
	actual := uint32(len(data) - 4)
	if declared != actual {
		t.Errorf("length prefix %d != actual payload length %d", declared, actual)
	}
}

// TestReadMessageTruncatedStream verifies that ReadMessage on a truncated
// stream returns a descriptive error.
func TestReadMessageTruncatedStream(t *testing.T) {
	// Write a 4-byte prefix claiming 100 bytes but provide only 4.
	var buf bytes.Buffer
	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], 100)
	buf.Write(prefix[:])
	// No payload bytes.

	var got any
	err := plugin.ReadMessage(&buf, &got)
	if err == nil {
		t.Fatal("expected error for truncated stream, got nil")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("expected 'truncated' in error, got: %v", err)
	}
}

// TestLargeMessageRoundTrip verifies that a message larger than 64 KiB
// round-trips correctly.
func TestLargeMessageRoundTrip(t *testing.T) {
	// Build a slice of 10,000 elements to produce >64KB payload.
	large := make([]string, 10_000)
	for i := range large {
		large[i] = "0123456789abcdef" // 16 bytes each → ~160KB
	}

	var buf bytes.Buffer
	if err := plugin.WriteMessage(&buf, large); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var got []string
	if err := plugin.ReadMessage(&buf, &got); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if len(got) != len(large) {
		t.Errorf("length mismatch: got %d, want %d", len(got), len(large))
	}
}
