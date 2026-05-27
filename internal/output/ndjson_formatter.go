package output

import (
	"encoding/json"
	"io"
)

// NDJSONFormatter renders one JSON value per line. This is the explicit
// record-oriented formatter for paginated item streams and event streams.
type NDJSONFormatter struct{}

func (f *NDJSONFormatter) Format(w io.Writer, resp *Response, color bool) error {
	return writeNDJSONLines(w, resp.Body, color)
}

func (f *NDJSONFormatter) FormatValue(w io.Writer, value any, color bool) error {
	return writeNDJSONLines(w, value, color)
}

func writeNDJSONLines(w io.Writer, value any, color bool) error {
	switch body := value.(type) {
	case []any:
		for _, item := range body {
			if err := writeCompactJSONLine(w, item, color); err != nil {
				return err
			}
		}
		return nil
	default:
		return writeCompactJSONLine(w, body, color)
	}
}

func (f *NDJSONFormatter) StartValueStream(w io.Writer, base *Response, color bool) (ValueStream, error) {
	return ndjsonValueStream{w: w, color: color}, nil
}

type ndjsonValueStream struct {
	w     io.Writer
	color bool
}

func (s ndjsonValueStream) WriteValue(value any) error {
	return writeCompactJSONLine(s.w, value, s.color)
}

func (s ndjsonValueStream) Close() error {
	return nil
}

func writeCompactJSONLine(w io.Writer, value any, color bool) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if color {
		return highlight(w, ReadableLexer, encoded)
	}
	_, err = w.Write(encoded)
	return err
}
