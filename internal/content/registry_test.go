package content_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/content"
)

var reg = content.Default()

// roundTrip encodes v then decodes and returns the result.
func roundTrip(t *testing.T, mime string, v any) any {
	t.Helper()
	data, err := reg.Encode(mime, v)
	if err != nil {
		t.Fatalf("encode(%s): %v", mime, err)
	}
	out, err := reg.Decode(mime, data)
	if err != nil {
		t.Fatalf("decode(%s): %v", mime, err)
	}
	return out
}

func TestJSONRoundTrip(t *testing.T) {
	in := map[string]any{"name": "Alice", "age": float64(30)}
	out := roundTrip(t, "application/json", in)
	b1, _ := json.Marshal(in)
	b2, _ := json.Marshal(out)
	if string(b1) != string(b2) {
		t.Errorf("JSON round-trip mismatch:\n got  %s\n want %s", b2, b1)
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	in := map[string]any{"x": "hello", "y": float64(42)}
	out := roundTrip(t, "application/yaml", in)
	// YAML unmarshal produces map[string]any with string keys.
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	if m["x"] != "hello" {
		t.Errorf("x: got %v, want hello", m["x"])
	}
}

func TestCBORRoundTrip(t *testing.T) {
	in := map[string]any{"cbor": true}
	out := roundTrip(t, "application/cbor", in)
	// makeJSONSafe always produces map[string]any.
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", out)
	}
	if m["cbor"] != true {
		t.Errorf("cbor field: got %v, want true", m["cbor"])
	}
}

func TestMakeJSONSafeIntegerKeys(t *testing.T) {
	// Encode a CBOR map with integer keys, then decode and verify the keys
	// are converted to their string representation.
	encoded, err := reg.Encode("application/cbor", map[any]any{1: "one", 2: "two"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := reg.Decode("application/cbor", encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any after makeJSONSafe, got %T", out)
	}
	if m["1"] != "one" || m["2"] != "two" {
		t.Errorf("unexpected map contents: %v", m)
	}
}

func TestAcceptHeader(t *testing.T) {
	h := reg.AcceptHeader()
	// cbor q=0.9 must appear before json q=0.5
	iCBOR := strings.Index(h, "application/cbor")
	iJSON := strings.Index(h, "application/json")
	if iCBOR == -1 || iJSON == -1 {
		t.Fatalf("Accept header missing expected types: %q", h)
	}
	if iCBOR > iJSON {
		t.Errorf("cbor should appear before json in Accept header: %q", h)
	}
	iSSE := strings.Index(h, "text/event-stream")
	if iSSE == -1 {
		t.Fatalf("text/event-stream missing from Accept header: %q", h)
	}
	// text/* with lowest q must be last
	iText := strings.Index(h, "text/*")
	if iText == -1 {
		t.Fatalf("text/* missing from Accept header: %q", h)
	}
	if iSSE > iText {
		t.Errorf("text/event-stream should appear before text/* in Accept header: %q", h)
	}
	if iText < iJSON {
		t.Errorf("text/* should appear after json in Accept header: %q", h)
	}
}

func TestGzipDecompression(t *testing.T) {
	// Build a gzip-compressed JSON payload.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte(`{"compressed":true}`))
	gz.Close()

	rc, err := reg.Decompress("gzip", &buf)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != `{"compressed":true}` {
		t.Errorf("decompressed: %q", data)
	}
}

func TestUnknownContentTypeFallback(t *testing.T) {
	out, err := reg.Decode("application/octet-stream", []byte("raw bytes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := out.(string); !ok || s != "raw bytes" {
		t.Errorf("expected raw string fallback, got %T(%v)", out, out)
	}
}

func TestTextMIMEType(t *testing.T) {
	out, err := reg.Decode("text/plain", []byte("hello world"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s, ok := out.(string); !ok || s != "hello world" {
		t.Errorf("expected string, got %T(%v)", out, out)
	}
}

func TestFormEncoding(t *testing.T) {
	data, err := reg.Encode("application/x-www-form-urlencoded", map[string]any{
		"username": "alice",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got := string(data)
	if got != "password=secret&username=alice" && got != "username=alice&password=secret" {
		t.Fatalf("unexpected form body: %q", got)
	}
}

func TestMultipartEncodingIncludesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(path, []byte("hello upload"), 0o644); err != nil {
		t.Fatalf("write upload: %v", err)
	}

	data, contentType, err := reg.EncodeWithType("multipart/form-data", map[string]any{
		"name": "alice",
		"file": "@" + path,
	})
	if err != nil {
		t.Fatalf("encode with type: %v", err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse media type: %v", err)
	}
	reader := multipart.NewReader(bytes.NewReader(data), params["boundary"])

	parts := map[string]string{}
	filenames := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		parts[part.FormName()] = string(content)
		filenames[part.FormName()] = part.FileName()
	}

	if parts["name"] != "alice" {
		t.Fatalf("name part: got %q", parts["name"])
	}
	if parts["file"] != "hello upload" {
		t.Fatalf("file part: got %q", parts["file"])
	}
	if filenames["file"] != "upload.txt" {
		t.Fatalf("file name: got %q", filenames["file"])
	}
}
