package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
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

func TestCommandClientFetchAPISpecContextTimesOut(t *testing.T) {
	hostToPluginR, hostToPluginW := io.Pipe()
	pluginToHostR, pluginToHostW := io.Pipe()
	defer hostToPluginR.Close()
	defer hostToPluginW.Close()
	defer pluginToHostR.Close()
	defer pluginToHostW.Close()

	client := NewCommandClient(hostToPluginR, pluginToHostW)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := client.FetchAPISpecContext(ctx, "example", "staging")
		errCh <- err
	}()

	var req APISpecMsg
	if err := NewDecoder(pluginToHostR).ReadMessage(&req); err != nil {
		t.Fatalf("read API spec request: %v", err)
	}
	if req.Name != "example" || req.Profile != "staging" {
		t.Fatalf("request = %#v", req)
	}

	err := <-errCh
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out or was canceled") {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, loaded := client.specs.Load("example\x00staging"); loaded {
		t.Fatal("timed-out API spec request was not removed from pending map")
	}
}

func TestCommandPluginDocsJSONExamplesDecode(t *testing.T) {
	data, err := os.ReadFile("../site/content/en/docs/plugins/command-plugins.md")
	if err != nil {
		t.Fatalf("read docs: %v", err)
	}
	re := regexp.MustCompile("(?s)```json\\n(.*?)\\n```")
	matches := re.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		t.Fatal("expected JSON examples in command plugin docs")
	}
	for _, match := range matches {
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(match[1], &envelope); err != nil {
			t.Fatalf("decode envelope %s: %v", match[1], err)
		}
		switch envelope.Type {
		case MsgTypeHTTPRequest:
			var msg HTTPRequestMsg
			if err := json.Unmarshal(match[1], &msg); err != nil {
				t.Fatalf("decode HTTPRequestMsg %s: %v", match[1], err)
			}
			if msg.Method == "" || msg.URI == "" {
				t.Fatalf("incomplete HTTP request example: %#v", msg)
			}
		case MsgTypeAPISpec:
			var msg APISpecMsg
			if err := json.Unmarshal(match[1], &msg); err != nil {
				t.Fatalf("decode APISpecMsg %s: %v", match[1], err)
			}
			if msg.Name == "" || msg.Profile == "" {
				t.Fatalf("incomplete API spec example: %#v", msg)
			}
		default:
			t.Fatalf("unexpected command plugin docs message type %q", envelope.Type)
		}
	}
}
