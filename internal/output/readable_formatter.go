package output

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
)


// ReadableFormatter writes the full response (status line, headers, body) in
// a human-friendly format. When color is true, the output is syntax-highlighted
// using chroma with restishStyle. The body is always valid JSON so it can be
// copied and pasted directly into other tools.
type ReadableFormatter struct{}

func (f *ReadableFormatter) Format(w io.Writer, resp *Response, color bool) error {
	// Build plain-text preamble: status line + headers.
	var preamble strings.Builder
	fmt.Fprintf(&preamble, "%s %d %s\n", resp.Proto, resp.Status, http.StatusText(resp.Status))

	keys := make([]string, 0, len(resp.Headers))
	for k := range resp.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&preamble, "%s: %s\n", k, resp.Headers[k])
	}

	if color {
		if err := highlight(w, HTTPPreambleLexer, []byte(preamble.String())); err != nil {
			return err
		}
	} else {
		if _, err := io.WriteString(w, preamble.String()); err != nil {
			return err
		}
	}

	fmt.Fprintln(w) // blank line between headers and body

	// Body.
	if resp.Body == nil {
		return nil
	}

	data, err := json.MarshalIndent(resp.Body, "", "  ")
	if err != nil {
		return fmt.Errorf("formatting body: %w", err)
	}
	data = append(data, '\n')

	if !color {
		_, err = w.Write(data)
		return err
	}

	readableIndentDepth = 0
	return highlight(w, ReadableLexer, data)
}

// highlight tokenizes data with the given lexer and writes chroma-colored
// output to w using restishStyle and true-color ANSI sequences.
// Falls back to plain output on any error.
func highlight(w io.Writer, lexer chroma.Lexer, data []byte) error {
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		_, err := w.Write(data)
		return err
	}

	iter, err := lexer.Tokenise(nil, string(data))
	if err != nil {
		_, werr := w.Write(data)
		return werr
	}

	return formatter.Format(w, restishStyle, iter)
}
