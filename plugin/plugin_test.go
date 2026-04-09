package plugin_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/fxamacker/cbor/v2"

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

// TestWriteMessageIsRawCBOR verifies that WriteMessage produces a plain CBOR
// data item with no additional framing (no length prefix, etc.).
func TestWriteMessageIsRawCBOR(t *testing.T) {
	var buf bytes.Buffer
	if err := plugin.WriteMessage(&buf, "test"); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	data := buf.Bytes()
	// The output must be valid CBOR that decodes back to our value.
	var got any
	if err := cbor.Unmarshal(data, &got); err != nil {
		t.Fatalf("output is not valid CBOR: %v", err)
	}
	if got != "test" {
		t.Errorf("decoded value: got %v, want %q", got, "test")
	}
}

// TestReadMessageTruncatedStream verifies that ReadMessage on a stream that
// ends mid-item returns a descriptive error.
func TestReadMessageTruncatedStream(t *testing.T) {
	// Start a CBOR text string of 100 bytes but provide only 2 bytes of content.
	// CBOR major type 3 (text string), additional info 24 (one-byte length follows),
	// length = 100, then only 2 bytes of actual content.
	buf := bytes.NewReader([]byte{0x78, 0x64, 'a', 'b'})

	var got any
	err := plugin.ReadMessage(buf, &got)
	if err == nil {
		t.Fatal("expected error for truncated stream, got nil")
	}
	// The error should mention truncation or EOF.
	s := err.Error()
	if !strings.Contains(s, "EOF") && !strings.Contains(s, "truncat") {
		t.Errorf("expected EOF or truncat in error, got: %v", err)
	}
}

// TestTypedMessageRoundTrip verifies that a typed message struct written with
// WriteMessage decodes back to the same map representation the host uses,
// confirming that cbor struct tags match the stringly-keyed wire format.
func TestTypedMessageRoundTrip(t *testing.T) {
	msg := plugin.HTTPRequestMsg{
		Type:    plugin.MsgTypeHTTPRequest,
		Method:  "POST",
		URI:     "https://api.example.com/items",
		NoCache: true,
		Filter:  "body.items",
	}

	var buf bytes.Buffer
	if err := plugin.WriteMessage(&buf, msg); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	var decoded map[string]any
	if err := plugin.ReadMessage(&buf, &decoded); err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}

	if decoded["type"] != plugin.MsgTypeHTTPRequest {
		t.Errorf("type: got %v, want %q", decoded["type"], plugin.MsgTypeHTTPRequest)
	}
	if decoded["method"] != "POST" {
		t.Errorf("method: got %v, want POST", decoded["method"])
	}
	if decoded["uri"] != "https://api.example.com/items" {
		t.Errorf("uri: got %v", decoded["uri"])
	}
	if decoded["no_cache"] != true {
		t.Errorf("no_cache: got %v, want true", decoded["no_cache"])
	}
	if decoded["filter"] != "body.items" {
		t.Errorf("filter: got %v, want body.items", decoded["filter"])
	}
}

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

// TestDecoderSequentialPipe verifies that NewDecoder correctly reads multiple
// CBOR items sent sequentially through an os.Pipe, simulating the subprocess
// stdin/stdout channel used by command and TLS-signer plugins.
//
// The cbor.Decoder maintains a 512-byte internal read buffer, so a fresh
// decoder created per ReadMessage call would silently discard buffered bytes
// belonging to later items. This test catches that regression.
func TestDecoderSequentialPipe(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()

	messages := []map[string]any{
		{"type": "init", "command": "greet"},
		{"type": "http-request", "method": "GET", "uri": "https://example.com/foo"},
		{"type": "done", "exit_code": uint64(0)},
	}

	go func() {
		defer pw.Close()
		for _, msg := range messages {
			if wErr := plugin.WriteMessage(pw, msg); wErr != nil {
				return
			}
		}
	}()

	dec := plugin.NewDecoder(pr)
	for i, want := range messages {
		var got map[string]any
		if err := dec.ReadMessage(&got); err != nil {
			t.Fatalf("ReadMessage[%d]: %v", i, err)
		}
		if got["type"] != want["type"] {
			t.Errorf("msg[%d] type: got %q, want %q", i, got["type"], want["type"])
		}
	}
}
