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

// DefaultFormatters returns the built-in set of formatters.
// The "table" entry here uses default (zero-value) column settings;
// callers that need --rsh-columns / --rsh-sort-by should replace it.
func DefaultFormatters() map[string]Formatter {
	return map[string]Formatter{
		"json":     &JSONFormatter{},
		"raw":      &RawFormatter{},
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
//   - Non-TTY default: "raw" (original bytes, safe for pipes and file redirects).
func Select(fmts map[string]Formatter, fmtName string, tty bool) (Formatter, bool) {
	if fmtName != "" {
		f, ok := fmts[fmtName]
		return f, ok
	}
	if tty {
		return fmts["readable"], true
	}
	return fmts["raw"], true
}
