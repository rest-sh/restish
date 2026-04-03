package output

import (
	"encoding/json"
	"io"
)

// JSONFormatter writes only the response body as indented JSON.
// This is the default format in non-interactive (pipe/file) mode.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(w io.Writer, resp *Response, color bool) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp.Body)
}
