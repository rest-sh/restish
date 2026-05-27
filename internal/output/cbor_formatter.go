package output

import (
	"io"

	"github.com/fxamacker/cbor/v2"
)

// CBORFormatter encodes the response body as CBOR and writes the raw bytes
// to stdout. Useful for feeding binary-safe pipelines.
type CBORFormatter struct{}

func (f *CBORFormatter) Format(w io.Writer, resp *Response, color bool) error {
	data, err := cbor.Marshal(resp.Body)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
