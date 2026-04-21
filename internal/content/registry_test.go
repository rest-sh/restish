package content_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
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

func TestAcceptHeaderCacheInvalidatesOnNewRegistration(t *testing.T) {
	r := content.New()
	r.AddContentType(&content.ContentType{
		Name:      "json",
		MIMETypes: []string{"application/json"},
		Quality:   0.5,
	})
	if got := r.AcceptHeader(); got != "application/json;q=0.5" {
		t.Fatalf("AcceptHeader() = %q, want %q", got, "application/json;q=0.5")
	}

	r.AddContentType(&content.ContentType{
		Name:      "cbor",
		MIMETypes: []string{"application/cbor"},
		Quality:   0.9,
	})
	got := r.AcceptHeader()
	if !strings.Contains(got, "application/cbor") || !strings.Contains(got, "application/json") {
		t.Fatalf("expected both content types after cache invalidation, got %q", got)
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

func TestStructuredSyntaxSuffixes(t *testing.T) {
	cases := []struct {
		mime string
		body []byte
	}{
		{mime: "application/problem+json", body: []byte(`{"title":"bad"}`)},
		{mime: "application/hal+json", body: []byte(`{"_links":{"self":{"href":"/"}}}`)},
		{mime: "application/vnd.api+json", body: []byte(`{"data":{"type":"users","id":"1"}}`)},
		{mime: "application/ld+json", body: []byte(`{"@context":"https://schema.org","@type":"Thing"}`)},
		{mime: "application/fhir+cbor", body: mustEncode(t, "application/cbor", map[string]any{"resourceType": "Patient"})},
	}

	for _, tc := range cases {
		t.Run(tc.mime, func(t *testing.T) {
			out, err := reg.Decode(tc.mime, tc.body)
			if err != nil {
				t.Fatalf("Decode(%s): %v", tc.mime, err)
			}
			if _, ok := out.(map[string]any); !ok {
				t.Fatalf("expected map[string]any, got %T", out)
			}
		})
	}
}

func TestUnknownBinaryFallbackEncodesAsBase64InJSON(t *testing.T) {
	data := []byte{0x00, 0xff, 0x10, 0x80}
	out, err := reg.Decode("application/x-octet-thing", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, ok := out.([]byte)
	if !ok {
		t.Fatalf("expected []byte fallback, got %T", out)
	}
	if !bytes.Equal(b, data) {
		t.Fatalf("decoded bytes = %v, want %v", b, data)
	}
	encoded, err := json.Marshal(map[string]any{"body": out})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	want := `{"body":"` + base64.StdEncoding.EncodeToString(data) + `"}`
	if string(encoded) != want {
		t.Fatalf("JSON = %s, want %s", encoded, want)
	}
}

func mustEncode(t *testing.T, mime string, v any) []byte {
	t.Helper()
	data, err := reg.Encode(mime, v)
	if err != nil {
		t.Fatalf("Encode(%s): %v", mime, err)
	}
	return data
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

// jsonRoundTrip is a test helper that serialises v to JSON and back, so
// that test assertions can compare normalised representations instead of
// direct struct equality (which fails across CBOR/msgpack integer widths).
func jsonRoundTrip(t *testing.T, v any) any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("jsonRoundTrip marshal: %v", err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("jsonRoundTrip unmarshal: %v", err)
	}
	return out
}

// TestCBORRoundTripJSONNormalized verifies CBOR round-trip using JSON
// normalisation so integer-width differences don't cause failures.
func TestCBORRoundTripJSONNormalized(t *testing.T) {
	in := map[string]any{"count": float64(42), "active": true, "label": "hello"}
	out := roundTrip(t, "application/cbor", in)
	got, _ := json.Marshal(jsonRoundTrip(t, out))
	want, _ := json.Marshal(jsonRoundTrip(t, in))
	if string(got) != string(want) {
		t.Errorf("CBOR round-trip mismatch:\n got  %s\n want %s", got, want)
	}
}

// TestCBORByteString verifies that CBOR byte strings survive the round-trip
// and are represented as []byte (base64 via JSON) rather than corrupted strings.
func TestCBORByteString(t *testing.T) {
	payload := []byte{0x00, 0x01, 0x02, 0xfe, 0xff}
	in := map[string]any{"data": payload}
	out := roundTrip(t, "application/cbor", in)
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	switch v := m["data"].(type) {
	case []byte:
		if string(v) != string(payload) {
			t.Errorf("byte value mismatch: got %v, want %v", v, payload)
		}
	default:
		// If it comes back as base64 string (after JSON encode), decode and check.
		s, ok := m["data"].(string)
		if !ok {
			t.Fatalf("expected []byte or string, got %T", m["data"])
		}
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			t.Fatalf("base64 decode: %v", err)
		}
		if string(decoded) != string(payload) {
			t.Errorf("decoded mismatch: got %v, want %v", decoded, payload)
		}
	}
}

// TestMsgpackRoundTripJSONNormalized verifies msgpack round-trip with JSON
// normalisation.
func TestMsgpackRoundTripJSONNormalized(t *testing.T) {
	in := map[string]any{"n": float64(7), "flag": false}
	out := roundTrip(t, "application/msgpack", in)
	got, _ := json.Marshal(jsonRoundTrip(t, out))
	want, _ := json.Marshal(jsonRoundTrip(t, in))
	if string(got) != string(want) {
		t.Errorf("msgpack round-trip mismatch:\n got  %s\n want %s", got, want)
	}
}
