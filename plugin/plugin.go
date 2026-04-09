// Package plugin provides helpers for writing Restish out-of-process plugins.
//
// Restish communicates with plugins using CBOR messages over stdin/stdout.
// Each message is a single self-delimiting CBOR data item — no length prefix
// or other framing is added. This means any CBOR library can read and write
// plugin messages without implementing custom framing.
//
// Use WriteMessage to write one message and ReadMessage to read one message.
// Both functions are safe for concurrent use on independent readers/writers.
package plugin

import (
	"fmt"
	"io"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

// DecMode is a CBOR decode mode configured to use map[string]any for all CBOR
// maps (including nested ones), rather than the default map[interface{}]interface{}.
// Plugin authors can use it to decode CBOR payloads without reimplementing the
// same DecOptions.
var DecMode = func() cbor.DecMode {
	dm, err := cbor.DecOptions{
		DefaultMapType: reflect.TypeOf(map[string]any{}),
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

// ReadMessage reads one CBOR data item from r and unmarshals it into v (which
// must be a pointer). Because CBOR is self-delimiting, ReadMessage consumes
// exactly the bytes that belong to the next data item and no more.
//
// ReadMessage is safe to call repeatedly on the same reader: each call
// advances r past exactly one CBOR item regardless of what follows.
func ReadMessage(r io.Reader, v any) error {
	if err := DecMode.NewDecoder(r).Decode(v); err != nil {
		return fmt.Errorf("plugin: read: %w", err)
	}
	return nil
}
