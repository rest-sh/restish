// Package content provides a registry of content types and encodings for
// marshalling/unmarshalling request and response bodies, and for transparent
// decompression of compressed responses.
package content

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/andybalholm/brotli"
)

// ContentType describes how to marshal and unmarshal a single MIME type.
type ContentType struct {
	// Name is a short identifier used in CLI flags (e.g. "json", "yaml").
	Name string
	// MIMETypes lists all MIME types this entry handles (e.g. "application/json").
	// A trailing "/*" is treated as a wildcard prefix match.
	MIMETypes []string
	// Suffixes lists structured syntax suffixes handled by this entry, such as
	// "+json" or "+cbor".
	Suffixes []string
	// Quality is the Accept header q-value (0–1). Higher = preferred.
	Quality float32
	// Marshal encodes v into bytes.
	Marshal func(v any) ([]byte, error)
	// MarshalContentType optionally encodes v and returns the concrete
	// Content-Type to send. This is useful for formats like multipart where
	// runtime parameters (for example a boundary) are required.
	MarshalContentType func(v any) ([]byte, string, error)
	// Unmarshal decodes data into a Go value.
	Unmarshal func(data []byte) (any, error)
}

// Encoding describes how to decompress a single Content-Encoding.
type Encoding struct {
	// Name is the encoding token used in Accept-Encoding / Content-Encoding.
	Name string
	// Quality is the Accept-Encoding q-value.
	Quality float32
	// Decompress wraps r with a decompressing reader.
	Decompress func(r io.Reader) (io.ReadCloser, error)
}

// Registry holds the set of known content types and encodings.
type Registry struct {
	contentTypes []*ContentType
	encodings    []*Encoding

	headerMu             sync.Mutex
	acceptHeader         string
	acceptHeaderReady    bool
	acceptEncodingHeader string
	acceptEncodingReady  bool
}

// DisplayRanges includes all viewable Unicode characters along with white
// space.
var DisplayRanges = []*unicode.RangeTable{
	unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.White_Space,
}

// New returns a Registry with no registrations.
func New() *Registry {
	return &Registry{}
}

// AddContentType registers a content type. Later registrations take precedence
// for MIME-type matching when two entries share the same type.
func (r *Registry) AddContentType(ct *ContentType) {
	r.contentTypes = append(r.contentTypes, ct)
	r.headerMu.Lock()
	r.acceptHeader = ""
	r.acceptHeaderReady = false
	r.headerMu.Unlock()
}

// ContentTypes returns all registered content types in registration order.
func (r *Registry) ContentTypes() []*ContentType {
	return r.contentTypes
}

// AddEncoding registers a compression encoding.
func (r *Registry) AddEncoding(e *Encoding) {
	r.encodings = append(r.encodings, e)
	r.headerMu.Lock()
	r.acceptEncodingHeader = ""
	r.acceptEncodingReady = false
	r.headerMu.Unlock()
}

// qualityEntry pairs a token name with its negotiation quality value.
type qualityEntry struct {
	name string
	q    float32
}

// buildQualityHeader sorts entries by quality descending and formats them as
// a comma-separated Accept / Accept-Encoding header value.
func buildQualityHeader(entries []qualityEntry) string {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].q > entries[j].q
	})
	parts := make([]string, len(entries))
	for i, e := range entries {
		if e.q == 1.0 {
			parts[i] = e.name
		} else {
			parts[i] = fmt.Sprintf("%s;q=%.1f", e.name, e.q)
		}
	}
	return strings.Join(parts, ", ")
}

// AcceptHeader returns a sorted Accept header value built from all registered
// content types, ordered by quality descending.
func (r *Registry) AcceptHeader() string {
	r.headerMu.Lock()
	defer r.headerMu.Unlock()
	if r.acceptHeaderReady {
		return r.acceptHeader
	}
	var entries []qualityEntry
	for _, ct := range r.contentTypes {
		for _, mt := range ct.MIMETypes {
			entries = append(entries, qualityEntry{mt, ct.Quality})
		}
	}
	r.acceptHeader = buildQualityHeader(entries)
	r.acceptHeaderReady = true
	return r.acceptHeader
}

// AcceptEncodingHeader returns a sorted Accept-Encoding header value built
// from all registered encodings, ordered by quality descending.
func (r *Registry) AcceptEncodingHeader() string {
	r.headerMu.Lock()
	defer r.headerMu.Unlock()
	if r.acceptEncodingReady {
		return r.acceptEncodingHeader
	}
	entries := make([]qualityEntry, len(r.encodings))
	for i, e := range r.encodings {
		entries[i] = qualityEntry{e.Name, e.Quality}
	}
	r.acceptEncodingHeader = buildQualityHeader(entries)
	r.acceptEncodingReady = true
	return r.acceptEncodingHeader
}

// Decode finds the best-matching registered content type for mimeType,
// unmarshals data, and normalizes all map keys to strings so the result
// is always safe to pass to encoding/json. Returns the raw bytes as a
// string or []byte if no match is found.
func (r *Registry) Decode(mimeType string, data []byte) (any, error) {
	if strings.EqualFold(strings.TrimSpace(mimeType), "identity") {
		if b, ok := Printable(data); ok {
			return string(b), nil
		}
		return data, nil
	}
	ct := r.find(mimeType)
	if ct == nil {
		if b, ok := Printable(data); ok {
			return string(b), nil
		}
		return data, nil
	}
	v, err := ct.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return makeJSONSafe(v), nil
}

// makeJSONSafe recursively converts all map keys to strings so that the
// result can always be marshalled by encoding/json. Some decoders (e.g.
// CBOR, msgpack) produce map[interface{}]interface{} with non-string keys.
func makeJSONSafe(v any) any {
	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	switch val.Kind() {
	case reflect.Slice:
		if _, ok := val.Interface().([]byte); ok {
			return val.Interface() // keep byte slices intact (base64 in JSON)
		}
		out := make([]any, val.Len())
		for i := range out {
			out[i] = makeJSONSafe(val.Index(i).Interface())
		}
		return out
	case reflect.Map:
		out := make(map[string]any, val.Len())
		for _, k := range val.MapKeys() {
			var key string
			if s, ok := k.Interface().(string); ok {
				key = s
			} else {
				key = fmt.Sprintf("%v", k.Interface())
			}
			out[key] = makeJSONSafe(val.MapIndex(k).Interface())
		}
		return out
	}
	return v
}

// Encode marshals v using the content type matching mimeType.
// Returns an error if no matching content type is registered.
func (r *Registry) Encode(mimeType string, v any) ([]byte, error) {
	data, _, err := r.EncodeWithType(mimeType, v)
	return data, err
}

// EncodeWithType marshals v using the content type matching mimeType and
// returns both the encoded bytes and the concrete Content-Type header value.
func (r *Registry) EncodeWithType(mimeType string, v any) ([]byte, string, error) {
	ct := r.find(mimeType)
	if ct == nil {
		return nil, "", fmt.Errorf("no encoder for content type %q", mimeType)
	}
	if ct.MarshalContentType != nil {
		data, concreteType, err := ct.MarshalContentType(v)
		if err != nil {
			return nil, "", err
		}
		if concreteType == "" {
			concreteType = mimeType
		}
		return data, concreteType, nil
	}
	data, err := ct.Marshal(v)
	if err != nil {
		return nil, "", err
	}
	return data, mimeType, nil
}

// Decompress wraps r with a decompressor for the named encoding.
// Returns r unchanged if encoding is empty or "identity".
func (r *Registry) Decompress(encoding string, reader io.Reader) (io.ReadCloser, error) {
	if encoding == "" || encoding == "identity" {
		return io.NopCloser(reader), nil
	}
	for _, e := range r.encodings {
		if strings.EqualFold(e.Name, encoding) {
			return e.Decompress(reader)
		}
	}
	return nil, fmt.Errorf("unsupported Content-Encoding %q", encoding)
}

// MIMETypeForName returns the primary MIME type for the content type registered
// under the given short name (e.g. "json" → "application/json"). Returns an
// empty string if no match is found.
func (r *Registry) MIMETypeForName(name string) string {
	for _, ct := range r.contentTypes {
		if ct.Name == name && len(ct.MIMETypes) > 0 {
			return ct.MIMETypes[0]
		}
	}
	return ""
}

// find returns the last-registered ContentType whose MIMETypes list contains
// a match for mimeType (exact or wildcard). Returns nil if none match.
func (r *Registry) find(mimeType string) *ContentType {
	base, _, _ := mime.ParseMediaType(mimeType)
	if base == "" {
		base = mimeType
	}

	var exactMatch *ContentType
	for _, ct := range r.contentTypes {
		for _, mt := range ct.MIMETypes {
			if strings.HasSuffix(mt, "/*") {
				continue
			}
			if strings.EqualFold(mt, base) {
				exactMatch = ct
			}
		}
	}
	if exactMatch != nil {
		return exactMatch
	}

	var wildcardMatch *ContentType
	for _, ct := range r.contentTypes {
		for _, mt := range ct.MIMETypes {
			if !strings.HasSuffix(mt, "/*") {
				continue
			}
			prefix := strings.TrimSuffix(mt, "*")
			if strings.HasPrefix(base, prefix) {
				wildcardMatch = ct
			}
		}
	}
	if wildcardMatch != nil {
		return wildcardMatch
	}

	var suffixMatch *ContentType
	for _, ct := range r.contentTypes {
		for _, suffix := range ct.Suffixes {
			if strings.HasSuffix(base, suffix) {
				suffixMatch = ct
			}
		}
	}
	return suffixMatch
}

// Printable returns body when it is likely safe and useful to display as text.
func Printable(body []byte) ([]byte, bool) {
	if len(body) >= 102400 || !utf8.Valid(body) {
		return nil, false
	}

	for i, r := range string(body) {
		if i == 0 && r == '\uFEFF' {
			continue
		}
		if i > 100 {
			break
		}
		if !unicode.In(r, DisplayRanges...) {
			return nil, false
		}
	}
	return body, true
}

// defaultBrotliDecompress wraps r with a brotli reader.
func defaultBrotliDecompress(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(brotli.NewReader(r)), nil
}

// defaultGzipDecompress wraps r with a gzip reader.
func defaultGzipDecompress(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

// defaultDeflateDecompress wraps r with a flate reader.
func defaultDeflateDecompress(r io.Reader) (io.ReadCloser, error) {
	return flate.NewReader(r), nil
}
