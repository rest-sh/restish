package plugin

import (
	"fmt"
	"io"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

const (
	maxCBORNestedLevels  = 16
	maxCBORArrayElements = 65536
	maxCBORMapPairs      = 16384
)

// DecMode is a CBOR decode mode configured to use map[string]any for all CBOR
// maps (including nested ones), rather than the default map[interface{}]interface{}.
// It also applies the same structural limits that Restish uses for plugin
// messages, so buggy plugins cannot force unbounded map, array, or nesting
// allocations before typed validation runs.
var DecMode = func() cbor.DecMode {
	dm, err := cbor.DecOptions{
		DefaultMapType:   reflect.TypeOf(map[string]any{}),
		MaxNestedLevels:  maxCBORNestedLevels,
		MaxArrayElements: maxCBORArrayElements,
		MaxMapPairs:      maxCBORMapPairs,
	}.DecMode()
	if err != nil {
		panic("plugin: creating CBOR decode mode: " + err.Error())
	}
	return dm
}()

// WriteMessage CBOR-encodes v and writes it to w as a single CBOR data item.
func WriteMessage(w io.Writer, v any) error {
	payload, err := cbor.Marshal(v)
	if err != nil {
		return fmt.Errorf("plugin: marshal: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("plugin: write: %w", err)
	}
	return nil
}

// Decoder reads sequential CBOR messages from a stream. It maintains an
// internal read buffer, so a single Decoder instance must be reused for all
// reads from the same underlying reader — discarding it between calls would
// lose bytes that were already buffered.
//
// Use NewDecoder for any reader from which multiple messages will be read
// (os.Stdin in a command or TLS-signer plugin). Hook plugins that receive
// exactly one message may use the package-level ReadMessage instead.
type Decoder struct {
	dec *cbor.Decoder
}

// NewDecoder returns a Decoder that reads from r.
//
// Reuse a single Decoder for the lifetime of the stream. Constructing a new
// Decoder inside a read loop can discard bytes that were already buffered and
// silently lose messages.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{dec: DecMode.NewDecoder(r)}
}

// ReadMessage reads one CBOR data item and unmarshals it into v (which must
// be a pointer).
func (d *Decoder) ReadMessage(v any) error {
	if err := d.dec.Decode(v); err != nil {
		return fmt.Errorf("plugin: read: %w", err)
	}
	return nil
}

// ReadRaw reads the next CBOR data item from the stream and returns it as raw
// bytes. The caller can inspect the bytes (e.g. with MessageType) and then
// decode into a specific typed struct using DecMode.Unmarshal.
func (d *Decoder) ReadRaw() ([]byte, error) {
	var raw cbor.RawMessage
	if err := d.dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("plugin: read: %w", err)
	}
	return raw, nil
}

// MessageType returns the "type" field of a raw CBOR message without fully
// decoding it. Returns an empty string if the field is absent or the data is
// not valid CBOR.
func MessageType(raw []byte) string {
	var env struct {
		Type string `cbor:"type"`
	}
	_ = DecMode.Unmarshal(raw, &env)
	return env.Type
}

// ReadMessage reads one CBOR data item from r and unmarshals it into v (which
// must be a pointer). It is intended for one-shot reads such as hook plugins.
// For command or TLS-signer plugins that receive multiple messages, create a
// Decoder with NewDecoder and call ReadMessage on that instead.
func ReadMessage(r io.Reader, v any) error {
	return NewDecoder(r).ReadMessage(v)
}
