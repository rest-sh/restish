package output

import (
	"encoding/json"
	"io"
)

// NDJSONFormatter renders one JSON value per line. This is the explicit
// record-oriented formatter for paginated item streams and event streams.
type NDJSONFormatter struct{}

func (f *NDJSONFormatter) Format(w io.Writer, resp *Response, color bool) error {
	switch body := resp.Body.(type) {
	case []any:
		for _, item := range body {
			if err := writeCompactJSONLine(w, item); err != nil {
				return err
			}
		}
		return nil
	default:
		return writeCompactJSONLine(w, body)
	}
}

func (f *NDJSONFormatter) FormatValue(w io.Writer, value any, color bool) error {
	return writeCompactJSONLine(w, value)
}

func (f *NDJSONFormatter) StartValueStream(w io.Writer, base *Response, color bool) (ValueStream, error) {
	return ndjsonValueStream{w: w}, nil
}

type ndjsonValueStream struct {
	w io.Writer
}

func (s ndjsonValueStream) WriteValue(value any) error {
	return writeCompactJSONLine(s.w, value)
}

func (s ndjsonValueStream) Close() error {
	return nil
}

func writeCompactJSONLine(w io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	_, err = w.Write(encoded)
	return err
}
