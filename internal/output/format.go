package output

import "io"

// Formatter renders a normalized Response to a writer.
// color indicates whether ANSI escape sequences are appropriate.
type Formatter interface {
	Format(w io.Writer, resp *Response, color bool) error
}

// DefaultFormatters returns the built-in set of formatters.
// Additional formatters are registered on the CLI struct (Step 21+).
func DefaultFormatters() map[string]Formatter {
	return map[string]Formatter{
		"json":     &JSONFormatter{},
		"raw":      &RawFormatter{},
		"readable": &ReadableFormatter{},
	}
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
