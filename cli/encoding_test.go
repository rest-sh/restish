package cli

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
)

func gzipEnc(data string) []byte {
	b := bytes.NewBuffer(nil)
	w := gzip.NewWriter(b)
	w.Write([]byte(data))
	w.Close()
	return b.Bytes()
}

func deflateEnc(data string) []byte {
	b := bytes.NewBuffer(nil)
	w, _ := flate.NewWriter(b, 1)
	w.Write([]byte(data))
	w.Close()
	return b.Bytes()
}

func brEnc(data string) []byte {
	b := bytes.NewBuffer(nil)
	w := brotli.NewWriter(b)
	w.Write([]byte(data))
	w.Close()
	return b.Bytes()
}

var encodingTests = []struct {
	name   string
	header string
	data   []byte
}{
	{"none", "", []byte("hello world")},
	{"gzip", "gzip", gzipEnc("hello world")},
	{"deflate", "deflate", deflateEnc("hello world")},
	{"brotli", "br", brEnc("hello world")},
}

func TestEncodings(parent *testing.T) {
	AddEncoding("deflate", &DeflateEncoding{})
	AddEncoding("gzip", &GzipEncoding{})
	AddEncoding("br", &BrotliEncoding{})
	parent.Cleanup(func() {
		encodings = nil
	})

	for _, tt := range encodingTests {
		parent.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: http.Header{
					"Content-Encoding": []string{tt.header},
				},
				Body: io.NopCloser(bytes.NewReader(tt.data)),
			}

			content, err := DecodeResponse(resp)
			assert.NoError(t, err)

			data, err := io.ReadAll(content)
			assert.NoError(t, err)
			assert.Equal(t, "hello world", string(data))
		})
	}
}
