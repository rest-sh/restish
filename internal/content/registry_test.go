package content_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/rest-sh/restish/v2/internal/content"
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

func TestXMLEncodesRawTextAndRejectsStructuredValues(t *testing.T) {
	data, err := reg.Encode("application/xml", `<propfind xmlns="DAV:"><prop><displayname/></prop></propfind>`)
	if err != nil {
		t.Fatalf("encode xml string: %v", err)
	}
	if got, want := string(data), `<propfind xmlns="DAV:"><prop><displayname/></prop></propfind>`; got != want {
		t.Fatalf("XML string encode = %q, want %q", got, want)
	}
	data, err = reg.Encode("application/merge+xml", []byte(`<merge/>`))
	if err != nil {
		t.Fatalf("encode +xml bytes: %v", err)
	}
	if got, want := string(data), `<merge/>`; got != want {
		t.Fatalf("+xml bytes encode = %q, want %q", got, want)
	}
	_, err = reg.Encode("application/xml", map[string]any{"displayname": true})
	if err == nil {
		t.Fatal("expected structured XML encode error")
	}
	if !strings.Contains(err.Error(), "XML request bodies must be supplied as raw text or @file input") {
		t.Fatalf("unexpected XML encode error: %v", err)
	}
}

func TestNDJSONPreservesRawTextInput(t *testing.T) {
	raw := "{\"message\":\"one\"}\n{\"message\":\"two\"}\n"
	data, err := reg.Encode("application/x-ndjson", raw)
	if err != nil {
		t.Fatalf("encode ndjson string: %v", err)
	}
	if got := string(data); got != raw {
		t.Fatalf("NDJSON string encode = %q, want raw input %q", got, raw)
	}
	data, err = reg.Encode("application/jsonl", []byte(raw))
	if err != nil {
		t.Fatalf("encode jsonl bytes: %v", err)
	}
	if got := string(data); got != raw {
		t.Fatalf("JSONL bytes encode = %q, want raw input %q", got, raw)
	}
}

func TestDecodeIdentityContentType(t *testing.T) {
	out, err := reg.Decode("identity", []byte("plain response"))
	if err != nil {
		t.Fatalf("decode identity: %v", err)
	}
	if out != "plain response" {
		t.Fatalf("identity decode = %#v, want plain response", out)
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
	// JSON is the least surprising default negotiation target. Binary
	// structured formats remain supported, but should not win by default.
	iCBOR := strings.Index(h, "application/cbor")
	iJSON := strings.Index(h, "application/json")
	iYAML := strings.Index(h, "application/yaml")
	if iCBOR == -1 || iJSON == -1 || iYAML == -1 {
		t.Fatalf("Accept header missing expected types: %q", h)
	}
	if iJSON > iYAML {
		t.Errorf("json should appear before yaml in Accept header: %q", h)
	}
	if iYAML > iCBOR {
		t.Errorf("yaml should appear before cbor in Accept header: %q", h)
	}
	if iJSON > iCBOR {
		t.Errorf("json should appear before cbor in Accept header: %q", h)
	}
	if !strings.Contains(h, "application/x-ndjson") {
		t.Fatalf("Accept header missing application/x-ndjson: %q", h)
	}
	if !strings.Contains(h, "application/x-ndjson;q=0.8") {
		t.Fatalf("Accept header should prefer NDJSON below JSON: %q", h)
	}
	if !strings.Contains(h, "application/yaml;q=0.8") {
		t.Fatalf("Accept header should prefer YAML below JSON: %q", h)
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
	iWildcard := strings.Index(h, "*/*;q=0.1")
	if iWildcard == -1 {
		t.Fatalf("*/* fallback missing from Accept header: %q", h)
	}
	if iWildcard < iText {
		t.Errorf("*/* fallback should appear after text/* in Accept header: %q", h)
	}
}

func TestAcceptHeaderDeduplicatesCanonicalMIMETypes(t *testing.T) {
	r := content.New()
	r.AddContentType(&content.ContentType{
		Name:      "json",
		MIMETypes: []string{"application/json"},
		Quality:   0.5,
	})
	r.AddContentType(&content.ContentType{
		Name:      "custom-json",
		MIMETypes: []string{"Application/JSON"},
		Quality:   0.7,
	})

	h := r.AcceptHeader()
	if strings.Count(strings.ToLower(h), "application/json") != 1 {
		t.Fatalf("AcceptHeader() should advertise one canonical application/json, got %q", h)
	}
	if !strings.Contains(h, "Application/JSON;q=0.7") {
		t.Fatalf("AcceptHeader() should use later registration quality/name, got %q", h)
	}
}

func TestStructuredSuffixPrecedesTextWildcard(t *testing.T) {
	out, err := reg.Decode("text/example+json", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatalf("Decode(text/example+json): %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("Decode(text/example+json) = %T, want map[string]any", out)
	}
	if m["ok"] != true {
		t.Fatalf("decoded JSON suffix body = %#v", out)
	}
}

func TestTextAliasUsesPlainText(t *testing.T) {
	if got, want := reg.MIMETypeForName("text"), "text/plain"; got != want {
		t.Fatalf("MIMETypeForName(text) = %q, want %q", got, want)
	}
	if got, want := reg.MIMETypeForName("sse"), "text/event-stream"; got != want {
		t.Fatalf("MIMETypeForName(sse) = %q, want %q", got, want)
	}
}

func TestDecodeNDJSON(t *testing.T) {
	out, err := reg.Decode("application/x-ndjson", []byte("{\"n\":1}\n{\"n\":2}\n"))
	if err != nil {
		t.Fatalf("Decode(application/x-ndjson): %v", err)
	}
	items, ok := out.([]any)
	if !ok {
		t.Fatalf("decoded NDJSON type = %T, want []any", out)
	}
	if len(items) != 2 {
		t.Fatalf("decoded NDJSON length = %d, want 2", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["n"] != float64(1) {
		t.Fatalf("first NDJSON item = %#v", items[0])
	}
}

func TestDecodeJSONSequence(t *testing.T) {
	out, err := reg.Decode("application/json", []byte("{\"n\":1}\n{\"n\":2}\n"))
	if err != nil {
		t.Fatalf("Decode(application/json sequence): %v", err)
	}
	items, ok := out.([]any)
	if !ok {
		t.Fatalf("decoded JSON sequence type = %T, want []any", out)
	}
	if len(items) != 2 {
		t.Fatalf("decoded JSON sequence length = %d, want 2", len(items))
	}
	first, ok := items[0].(map[string]any)
	if !ok || first["n"] != float64(1) {
		t.Fatalf("first JSON sequence item = %#v", items[0])
	}
}

func TestDecodeInvalidJSONStillFails(t *testing.T) {
	if _, err := reg.Decode("application/json", []byte("{\"n\":1}\nnot-json\n")); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestEncodeNDJSON(t *testing.T) {
	data, err := reg.Encode("application/x-ndjson", []map[string]int{{"n": 1}, {"n": 2}})
	if err != nil {
		t.Fatalf("Encode(application/x-ndjson): %v", err)
	}
	if got, want := string(data), "{\"n\":1}\n{\"n\":2}\n"; got != want {
		t.Fatalf("encoded NDJSON = %q, want %q", got, want)
	}
}

func TestAcceptHeaderCacheInvalidatesOnNewRegistration(t *testing.T) {
	r := content.New()
	r.AddContentType(&content.ContentType{
		Name:      "json",
		MIMETypes: []string{"application/json"},
		Quality:   0.5,
	})
	if got := r.AcceptHeader(); got != "application/json;q=0.5, */*;q=0.1" {
		t.Fatalf("AcceptHeader() = %q, want %q", got, "application/json;q=0.5, */*;q=0.1")
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

func TestAcceptHeaderForUsesRequestedSupportedMediaTypes(t *testing.T) {
	r := content.Default()
	got := r.AcceptHeaderFor([]string{
		"application/vnd.example+json",
		"application/cbor",
		"application/unknown",
		"text/plain",
		"application/json",
	})
	want := "application/vnd.example+json;q=0.9, application/json;q=0.9, application/cbor;q=0.6, text/plain;q=0.2"
	if got != want {
		t.Fatalf("AcceptHeaderFor() = %q, want %q", got, want)
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

func TestBuiltInDecompressors(t *testing.T) {
	encoders := map[string]func(string) []byte{
		"gzip": func(s string) []byte {
			var buf bytes.Buffer
			w := gzip.NewWriter(&buf)
			_, _ = w.Write([]byte(s))
			_ = w.Close()
			return buf.Bytes()
		},
		"deflate": func(s string) []byte {
			var buf bytes.Buffer
			w, err := flate.NewWriter(&buf, flate.DefaultCompression)
			if err != nil {
				t.Fatalf("flate writer: %v", err)
			}
			_, _ = w.Write([]byte(s))
			_ = w.Close()
			return buf.Bytes()
		},
		"br": func(s string) []byte {
			var buf bytes.Buffer
			w := brotli.NewWriter(&buf)
			_, _ = w.Write([]byte(s))
			_ = w.Close()
			return buf.Bytes()
		},
	}

	for encoding, encode := range encoders {
		t.Run(encoding, func(t *testing.T) {
			rc, err := reg.Decompress(encoding, bytes.NewReader(encode("hello world")))
			if err != nil {
				t.Fatalf("decompress %s: %v", encoding, err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read %s: %v", encoding, err)
			}
			if string(data) != "hello world" {
				t.Fatalf("%s decoded %q, want hello world", encoding, data)
			}
		})
	}
}

func TestDeflateDecompressionAcceptsZlibWrappedBody(t *testing.T) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, _ = w.Write([]byte("hello world"))
	_ = w.Close()

	rc, err := reg.Decompress("deflate", &buf)
	if err != nil {
		t.Fatalf("decompress deflate: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read deflate: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("deflate decoded %q, want hello world", data)
	}
}

func TestEncodeUnknownContentTypeErrors(t *testing.T) {
	if _, err := reg.Encode("application/unknown", map[string]any{"foo": 1}); err == nil {
		t.Fatal("expected unknown request content type to fail")
	}
}

func TestIdentityDecompressionNoOp(t *testing.T) {
	rc, err := reg.Decompress("identity", strings.NewReader("plain"))
	if err != nil {
		t.Fatalf("decompress identity: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read identity: %v", err)
	}
	if string(data) != "plain" {
		t.Fatalf("identity decoded %q, want plain", data)
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
		if errors.Is(err, io.EOF) {
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

func TestMultipartEncodingRejectsMissingFileReference(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.txt")
	_, _, err := reg.EncodeWithType("multipart/form-data", map[string]any{
		"file": "@" + missing,
	})
	if err == nil {
		t.Fatal("expected missing multipart file reference to fail")
	}
	if !strings.Contains(err.Error(), "unable to read multipart file") ||
		!strings.Contains(err.Error(), missing) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultipartEncodingRejectsDirectoryFileReference(t *testing.T) {
	dir := t.TempDir()
	_, _, err := reg.EncodeWithType("multipart/form-data", map[string]any{
		"file": "@" + dir,
	})
	if err == nil {
		t.Fatal("expected directory multipart file reference to fail")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultipartEncodingEscapesAtLiteral(t *testing.T) {
	data, contentType, err := reg.EncodeWithType("multipart/form-data", map[string]any{
		"note": "@@handle",
	})
	if err != nil {
		t.Fatalf("encode with type: %v", err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse media type: %v", err)
	}
	reader := multipart.NewReader(bytes.NewReader(data), params["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("next part: %v", err)
	}
	content, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("read part: %v", err)
	}
	if part.FormName() != "note" {
		t.Fatalf("form name: got %q", part.FormName())
	}
	if part.FileName() != "" {
		t.Fatalf("filename: got %q", part.FileName())
	}
	if string(content) != "@handle" {
		t.Fatalf("content: got %q", content)
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

func TestMsgpackMalformedFixextReturnsError(t *testing.T) {
	payloads := map[string][]byte{
		"fixext4 missing bytes": {0xd6, 0xff},
		"fixext8 missing bytes": {0xd7, 0xff},
	}
	for name, payload := range payloads {
		t.Run(name, func(t *testing.T) {
			if _, err := reg.Decode("application/msgpack", payload); err == nil {
				t.Fatal("expected malformed msgpack to return an error")
			}
		})
	}
}
