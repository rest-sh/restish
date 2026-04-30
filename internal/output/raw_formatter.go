package output

import "io"

// RawFormatter writes response body bytes after network transport/content
// encoding has been removed, without output formatter re-encoding. This is the
// default for non-TTY output (pipes, file redirects) so binary formats such as
// CBOR, images, and arbitrary payloads are preserved as decoded content bytes.
type RawFormatter struct{}

func (f *RawFormatter) Format(w io.Writer, resp *Response, _ bool) error {
	if len(resp.Raw) == 0 {
		return nil
	}
	_, err := w.Write(resp.Raw)
	return err
}
