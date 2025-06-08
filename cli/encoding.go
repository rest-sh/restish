package cli

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

// ContentEncoding is used to encode/decode content for transfer over the wire,
// for example with gzip.
type ContentEncoding interface {
	Reader(stream io.Reader) (io.Reader, error)
}

// contentTypes is a list of acceptable content types
var encodings = map[string]ContentEncoding{}

// AddEncoding adds a new content encoding with the given name.
func AddEncoding(name string, encoding ContentEncoding) {
	encodings[name] = encoding
}

func buildAcceptEncodingHeader() string {
	accept := []string{}

	for name := range encodings {
		accept = append(accept, name)
	}

	return strings.Join(accept, ", ")
}

// DecodeResponse will return a reader to decode the response body based on the encoding.
// Assumes the original body will be closed outside of this function.
func DecodeResponse(resp *http.Response) (io.Reader, error) {
	contentEncoding := resp.Header.Get("content-encoding")

	// The net/http package handles decompressing responses in some cases.
	// When it does, it deletes the content-encoding and content-length headers.
	// This handles pre-decoded and non-encoded responses.
	if contentEncoding == "" {
		return resp.Body, nil
	}

	encoding := encodings[contentEncoding]

	if encoding == nil {
		return nil, fmt.Errorf("unsupported content-encoding %s", contentEncoding)
	}

	LogDebug("Decoding response from %s", contentEncoding)

	reader, err := encoding.Reader(resp.Body)
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// DeflateEncoding supports gzip-encoded response content.
type DeflateEncoding struct{}

// Reader returns a new reader for the stream that removes the gzip encoding.
func (g DeflateEncoding) Reader(stream io.Reader) (io.Reader, error) {
	return flate.NewReader(stream), nil
}

// GzipEncoding supports gzip-encoded response content.
type GzipEncoding struct{}

// Reader returns a new reader for the stream that removes the gzip encoding.
func (g GzipEncoding) Reader(stream io.Reader) (io.Reader, error) {
	return gzip.NewReader(stream)
}

// BrotliEncoding supports RFC 7932 Brotli content encoding.
type BrotliEncoding struct{}

// Reader returns a new reader for the stream that removes the brotli encoding.
func (b BrotliEncoding) Reader(stream io.Reader) (io.Reader, error) {
	return io.Reader(brotli.NewReader(stream)), nil
}
