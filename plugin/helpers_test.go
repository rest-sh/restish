package plugin

import (
	"bytes"
	"testing"
)

func TestMsgBytesFloat64Elements(t *testing.T) {
	got := MsgBytes([]any{float64(65), float64(66), float64(67)})
	if !bytes.Equal(got, []byte("ABC")) {
		t.Fatalf("MsgBytes float64 elements = %v, want ABC", got)
	}
}

func TestMsgBytesSkipsInvalidFloat64Elements(t *testing.T) {
	got := MsgBytes([]any{float64(65.5), float64(-1), float64(256), float64(66)})
	if !bytes.Equal(got, []byte("B")) {
		t.Fatalf("MsgBytes invalid float64 elements = %v, want B", got)
	}
}
