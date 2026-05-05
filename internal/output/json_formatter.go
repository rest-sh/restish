package output

import (
	"bytes"
	"encoding/json"
	"io"
)

// JSONFormatter writes only the response body as indented JSON.
// This is the default format in non-interactive (pipe/file) mode.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(w io.Writer, resp *Response, color bool) error {
	data, err := marshalIndentNoEscape(resp.Body)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if color {
		return highlight(w, ReadableLexer, data)
	}
	_, err = w.Write(data)
	return err
}

func marshalNoEscape(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

func marshalIndentNoEscape(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}
