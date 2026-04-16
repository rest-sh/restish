package plugin

import (
	"bytes"
	"testing"
)

func TestCommandClientProgressWritesProgressMessage(t *testing.T) {
	var out bytes.Buffer
	client := NewCommandClient(bytes.NewReader(nil), &out)
	if err := client.Progress("working"); err != nil {
		t.Fatalf("Progress: %v", err)
	}

	var msg ProgressMsg
	if err := NewDecoder(bytes.NewReader(out.Bytes())).ReadMessage(&msg); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if msg.Type != MsgTypeProgress {
		t.Fatalf("Type = %q, want %q", msg.Type, MsgTypeProgress)
	}
	if msg.Text != "working" {
		t.Fatalf("Text = %q, want %q", msg.Text, "working")
	}
}
