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

func TestCommandClientWriteStdoutWritesStdoutDataMessage(t *testing.T) {
	var out bytes.Buffer
	client := NewCommandClient(bytes.NewReader(nil), &out)
	if err := client.WriteStdout([]byte("hello")); err != nil {
		t.Fatalf("WriteStdout: %v", err)
	}

	var msg StdoutDataMsg
	if err := NewDecoder(bytes.NewReader(out.Bytes())).ReadMessage(&msg); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if msg.Type != MsgTypeStdoutData || string(msg.Data) != "hello" {
		t.Fatalf("message = %#v", msg)
	}
}

func TestCommandClientWriteStderrWritesStderrDataMessage(t *testing.T) {
	var out bytes.Buffer
	client := NewCommandClient(bytes.NewReader(nil), &out)
	if err := client.WriteStderr([]byte("oops")); err != nil {
		t.Fatalf("WriteStderr: %v", err)
	}

	var msg StderrDataMsg
	if err := NewDecoder(bytes.NewReader(out.Bytes())).ReadMessage(&msg); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if msg.Type != MsgTypeStderrData || string(msg.Data) != "oops" {
		t.Fatalf("message = %#v", msg)
	}
}
