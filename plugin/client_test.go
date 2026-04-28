package plugin

import (
	"bytes"
	"io"
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

func TestCommandClientDoSkipsUnknownMessages(t *testing.T) {
	hostToPluginR, hostToPluginW := io.Pipe()
	pluginToHostR, pluginToHostW := io.Pipe()
	defer hostToPluginR.Close()
	defer hostToPluginW.Close()
	defer pluginToHostR.Close()
	defer pluginToHostW.Close()

	client := NewCommandClient(hostToPluginR, pluginToHostW)
	errCh := make(chan error, 1)
	go func() {
		resp, err := client.Do(&HTTPRequestMsg{URI: "https://api.example.com/items"})
		if err != nil {
			errCh <- err
			return
		}
		if resp.Status != 200 || resp.Body != "ok" {
			t.Errorf("unexpected response: %#v", resp)
		}
		errCh <- nil
	}()

	var req HTTPRequestMsg
	if err := NewDecoder(pluginToHostR).ReadMessage(&req); err != nil {
		t.Fatalf("read request: %v", err)
	}
	if err := WriteMessage(hostToPluginW, map[string]any{"type": "future-message"}); err != nil {
		t.Fatalf("write unknown message: %v", err)
	}
	if err := WriteMessage(hostToPluginW, HTTPResponseMsg{
		Type:      MsgTypeHTTPResponse,
		RequestID: req.RequestID,
		Status:    200,
		Body:      "ok",
	}); err != nil {
		t.Fatalf("write response: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("Do: %v", err)
	}
}
