package output

import (
	"io"
	"sort"
	"strings"
)

// Formatter renders a normalized Response to a writer.
// color indicates whether ANSI escape sequences are appropriate.
type Formatter interface {
	Format(w io.Writer, resp *Response, color bool) error
}

// ValueFormatter renders a body/sub-value without implying that it is a full
// HTTP response. This is used for filtered values, paginated item streams, and
// event streams where status/header preambles would be misleading.
type ValueFormatter interface {
	FormatValue(w io.Writer, value any, color bool) error
}

// ValueStream receives a sequence of body/sub-values for a single logical
// output session.
type ValueStream interface {
	WriteValue(value any) error
	Close() error
}

// ValueStreamFormatter can hold formatter-specific state across a stream of
// body/sub-values, such as writing one CSV header followed by many rows.
type ValueStreamFormatter interface {
	StartValueStream(w io.Writer, base *Response, color bool) (ValueStream, error)
}

// FramedValueTemplate describes how a streamed sequence of values should be
// embedded into a larger JSON document shape.
type FramedValueTemplate struct {
	Prefix      string
	Suffix      string
	ItemIndent  string
	CloseIndent string
}

// FramedValueStreamFormatter can render a sequence of body/sub-values into a
// larger framed document shape such as a JSON array inside an object.
type FramedValueStreamFormatter interface {
	StartFramedValueStream(w io.Writer, base *Response, color bool, frame FramedValueTemplate) (ValueStream, error)
}

// DefaultFormatters returns the built-in set of formatters.
// The "table" entry here uses default (zero-value) column settings;
// callers that need --rsh-columns / --rsh-sort-by should replace it.
func DefaultFormatters() map[string]Formatter {
	return map[string]Formatter{
		"json":     &JSONFormatter{},
		"lines":    &LinesFormatter{},
		"ndjson":   &NDJSONFormatter{},
		"readable": &ReadableFormatter{},
		"table":    &TableFormatter{},
		"gron":     &GronFormatter{},
		"cbor":     &CBORFormatter{},
		"image":    &ImageFormatter{},
		"yaml":     &YAMLFormatter{},
	}
}

// FormatterNames returns the sorted list of registered formatter names,
// suitable for use in help text and error messages.
func FormatterNames(fmts map[string]Formatter) string {
	names := make([]string, 0, len(fmts))
	for name := range fmts {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// Select picks the right formatter given an explicit format name and whether
// the output writer is a terminal.
//
//   - If fmtName is set and recognised, that formatter is returned.
//   - If fmtName is unrecognised, nil is returned so the caller can error.
//   - TTY default: "readable" (syntax-highlighted, human-friendly).
//   - Non-TTY default: "json" (structured output for pipes and file redirects).
func SelectDefault(fmts map[string]Formatter, tty bool) (Formatter, bool) {
	if tty {
		return fmts["readable"], true
	}
	return fmts["json"], true
}

func Select(fmts map[string]Formatter, fmtName string, tty bool) (Formatter, bool) {
	if fmtName != "" {
		f, ok := fmts[fmtName]
		return f, ok
	}
	return SelectDefault(fmts, tty)
}
