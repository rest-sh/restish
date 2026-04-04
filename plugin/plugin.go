// Package plugin provides helpers for writing Restish out-of-process plugins.
//
// Restish communicates with plugins using length-prefixed CBOR framing:
//
//	[ 4-byte big-endian uint32 length ][ CBOR payload of that many bytes ]
//
// Use WriteMessage to write one message and ReadMessage to read one message.
// Both functions are safe for concurrent use on independent readers/writers.
package plugin

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

// decMode is a CBOR decode mode configured to use map[string]any for all CBOR
// maps (including nested ones), rather than the default map[interface{}]interface{}.
var decMode = func() cbor.DecMode {
	dm, err := cbor.DecOptions{
		DefaultMapType: reflect.TypeOf(map[string]any{}),
	}.DecMode()
	if err != nil {
		panic("plugin: creating CBOR decode mode: " + err.Error())
	}
	return dm
}()

// maxMessageSize is the maximum allowed message size (64 MiB).
const maxMessageSize = 64 * 1024 * 1024

// WriteMessage CBOR-encodes v and writes it to w with a 4-byte big-endian
// length prefix.
func WriteMessage(w io.Writer, v any) error {
	payload, err := cbor.Marshal(v)
	if err != nil {
		return fmt.Errorf("plugin: marshal: %w", err)
	}
	if len(payload) > maxMessageSize {
		return fmt.Errorf("plugin: message too large (%d bytes, max %d)", len(payload), maxMessageSize)
	}

	var prefix [4]byte
	binary.BigEndian.PutUint32(prefix[:], uint32(len(payload)))
	if _, err := w.Write(prefix[:]); err != nil {
		return fmt.Errorf("plugin: write length prefix: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("plugin: write payload: %w", err)
	}
	return nil
}

// ReadMessage reads one length-prefixed CBOR message from r and unmarshals it
// into v (which must be a pointer).
func ReadMessage(r io.Reader, v any) error {
	var prefix [4]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		if err == io.ErrUnexpectedEOF {
			return fmt.Errorf("plugin: truncated length prefix")
		}
		return fmt.Errorf("plugin: read length prefix: %w", err)
	}

	size := binary.BigEndian.Uint32(prefix[:])
	if size == 0 {
		return fmt.Errorf("plugin: zero-length message")
	}
	if uint64(size) > math.MaxInt32 || int(size) > maxMessageSize {
		return fmt.Errorf("plugin: message length %d exceeds max %d", size, maxMessageSize)
	}

	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return fmt.Errorf("plugin: truncated payload (expected %d bytes)", size)
		}
		return fmt.Errorf("plugin: read payload: %w", err)
	}

	if err := decMode.Unmarshal(payload, v); err != nil {
		return fmt.Errorf("plugin: unmarshal: %w", err)
	}
	return nil
}
