package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// LinesFormatter renders scalar values as shell-friendly text, one value per
// line. Structured values are rejected so callers do not accidentally lose
// object or array shape.
type LinesFormatter struct{}

func (f *LinesFormatter) Format(w io.Writer, resp *Response, color bool) error {
	return f.FormatValue(w, resp.Body, color)
}

func (f *LinesFormatter) FormatValue(w io.Writer, value any, color bool) error {
	return WriteLinesValue(w, value)
}

func (f *LinesFormatter) StartValueStream(w io.Writer, base *Response, color bool) (ValueStream, error) {
	return linesValueStream{w: w}, nil
}

type linesValueStream struct {
	w io.Writer
}

func (s linesValueStream) WriteValue(value any) error {
	return WriteLinesValue(s.w, value)
}

func (s linesValueStream) Close() error {
	return nil
}

// IsLineScalar reports whether value can be rendered as one unquoted line.
func IsLineScalar(value any) bool {
	switch value.(type) {
	case nil,
		string,
		bool,
		json.Number,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64:
		return true
	default:
		return false
	}
}

// WriteLinesValue writes a scalar, or an array of scalars, as unquoted lines.
func WriteLinesValue(w io.Writer, value any) error {
	if items, ok := value.([]any); ok {
		for _, item := range items {
			if err := writeLineScalar(w, item); err != nil {
				return err
			}
		}
		return nil
	}
	return writeLineScalar(w, value)
}

func writeLineScalar(w io.Writer, value any) error {
	if !IsLineScalar(value) {
		return fmt.Errorf("lines: line output requires scalar values; use -o json for structured data")
	}
	var s string
	switch v := value.(type) {
	case nil:
		s = "null"
	case string:
		s = v
	default:
		s = fmt.Sprint(v)
	}
	if _, err := io.WriteString(w, s); err != nil {
		return err
	}
	if len(s) == 0 || s[len(s)-1] != '\n' {
		_, err := io.WriteString(w, "\n")
		return err
	}
	return nil
}
