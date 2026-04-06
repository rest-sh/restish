package plugin

import (
	"testing"
)

func TestMsgBytes_ByteSlice(t *testing.T) {
	input := []byte{1, 2, 3}
	got := MsgBytes(input)
	if string(got) != string(input) {
		t.Errorf("got %v, want %v", got, input)
	}
}

func TestMsgBytes_String(t *testing.T) {
	got := MsgBytes("hello")
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestMsgBytes_SliceOfUint64(t *testing.T) {
	input := []any{uint64(65), uint64(66), uint64(67)}
	got := MsgBytes(input)
	if string(got) != "ABC" {
		t.Errorf("got %q, want %q", got, "ABC")
	}
}

func TestMsgBytes_SliceOfInt64(t *testing.T) {
	input := []any{int64(72), int64(105)}
	got := MsgBytes(input)
	if string(got) != "Hi" {
		t.Errorf("got %q, want %q", got, "Hi")
	}
}

func TestMsgBytes_SliceOfInt(t *testing.T) {
	input := []any{int(79), int(75)}
	got := MsgBytes(input)
	if string(got) != "OK" {
		t.Errorf("got %q, want %q", got, "OK")
	}
}

func TestMsgBytes_Nil(t *testing.T) {
	got := MsgBytes(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestMsgBytes_UnknownType(t *testing.T) {
	got := MsgBytes(42) // int, not []any
	if got != nil {
		t.Errorf("expected nil for unknown type, got %v", got)
	}
}

func TestMsgBytes_EmptySlice(t *testing.T) {
	got := MsgBytes([]any{})
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}
