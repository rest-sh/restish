package output

import "io"

// RawFormatter writes the original response body bytes without any decoding
// or re-encoding. This is the default for non-TTY output (pipes, file
// redirects) so that binary formats such as CBOR, images, and arbitrary
// binary payloads are preserved exactly as received.
type RawFormatter struct{}

func (f *RawFormatter) Format(w io.Writer, resp *Response, _ bool) error {
	if len(resp.Raw) == 0 {
		return nil
	}
	_, err := w.Write(resp.Raw)
	return err
}
