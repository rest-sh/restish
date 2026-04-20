package output

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
)

// ReadableFormatter writes the full response (status line, headers, body) in
// a human-friendly format. When color is true, the output is syntax-highlighted
// using chroma with restishStyle. The body is always valid JSON so it can be
// copied and pasted directly into other tools.
type ReadableFormatter struct{}

func (f *ReadableFormatter) Format(w io.Writer, resp *Response, color bool) error {
	if err := writeHTTPPreamble(w, resp, color); err != nil {
		return err
	}

	fmt.Fprintln(w) // blank line between headers and body

	// Body.
	if resp.Body == nil {
		return nil
	}
	if strings.HasPrefix(resp.Headers["Content-Type"], "image/") && len(resp.Raw) > 0 {
		return (&ImageFormatter{}).Format(w, resp, color)
	}

	if data, ok := printableBody(resp); ok {
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		_, err := w.Write(data)
		return err
	}
	data, err := marshalIndentNoEscape(resp.Body)
	if err != nil {
		return fmt.Errorf("formatting body: %w", err)
	}
	data = append(data, '\n')

	if !color {
		_, err = w.Write(data)
		return err
	}

	return highlight(w, ReadableLexer, data)
}

// StartValueStream writes the HTTP preamble once, then renders each streamed
// item as a pretty-printed JSON block for fast human feedback on TTYs.
func (f *ReadableFormatter) StartValueStream(w io.Writer, base *Response, color bool) (ValueStream, error) {
	if base != nil && (base.Proto != "" || base.Status != 0 || len(base.Headers) > 0) {
		if err := writeHTTPPreamble(w, base, color); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return nil, err
		}
	}
	return &readableValueStream{w: w, color: color}, nil
}

// StartFramedValueStream writes the HTTP preamble once, then renders streamed
// items into a larger JSON document shape described by frame.
func (f *ReadableFormatter) StartFramedValueStream(w io.Writer, base *Response, color bool, frame FramedValueTemplate) (ValueStream, error) {
	if base != nil && (base.Proto != "" || base.Status != 0 || len(base.Headers) > 0) {
		if err := writeHTTPPreamble(w, base, color); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return nil, err
		}
	}
	return &readableFramedValueStream{
		w:     w,
		color: color,
		frame: frame,
	}, nil
}

type readableValueStream struct {
	w     io.Writer
	color bool
	first bool
}

func (s *readableValueStream) WriteValue(value any) error {
	if s.first {
		if _, err := io.WriteString(s.w, "\n"); err != nil {
			return err
		}
	}
	s.first = true

	data, err := marshalIndentNoEscape(value)
	if err != nil {
		return fmt.Errorf("formatting body: %w", err)
	}
	data = append(data, '\n')
	if s.color {
		return highlight(s.w, ReadableLexer, data)
	}
	_, err = s.w.Write(data)
	return err
}

func (s *readableValueStream) Close() error {
	return nil
}

type readableFramedValueStream struct {
	w         io.Writer
	color     bool
	frame     FramedValueTemplate
	started   bool
	wroteItem bool
}

func (s *readableFramedValueStream) WriteValue(value any) error {
	if !s.started {
		if _, err := io.WriteString(s.w, s.frame.Prefix+"["); err != nil {
			return err
		}
		s.started = true
	}

	item, err := marshalIndentNoEscape(value)
	if err != nil {
		return fmt.Errorf("formatting body: %w", err)
	}

	if s.wroteItem {
		if _, err := io.WriteString(s.w, ",\n"); err != nil {
			return err
		}
	} else {
		if _, err := io.WriteString(s.w, "\n"); err != nil {
			return err
		}
	}
	s.wroteItem = true

	item = indentBlock(item, s.frame.ItemIndent)
	if s.color {
		return highlight(s.w, ReadableLexer, item)
	}
	_, err = s.w.Write(item)
	return err
}

func (s *readableFramedValueStream) Close() error {
	if !s.started {
		if _, err := io.WriteString(s.w, s.frame.Prefix+"[]"+s.frame.Suffix+"\n"); err != nil {
			return err
		}
		return nil
	}

	if s.wroteItem {
		if _, err := io.WriteString(s.w, "\n"+s.frame.CloseIndent+"]"+s.frame.Suffix+"\n"); err != nil {
			return err
		}
		return nil
	}

	_, err := io.WriteString(s.w, "]"+s.frame.Suffix+"\n")
	return err
}

func indentBlock(data []byte, indent string) []byte {
	if len(data) == 0 {
		return []byte(indent)
	}
	var out bytes.Buffer
	out.Grow(len(data) + len(indent))
	lineStart := 0
	for i := 0; i <= len(data); i++ {
		if i < len(data) && data[i] != '\n' {
			continue
		}
		out.WriteString(indent)
		out.Write(data[lineStart:i])
		if i < len(data) {
			out.WriteByte('\n')
		}
		lineStart = i + 1
	}
	return out.Bytes()
}

func writeHTTPPreamble(w io.Writer, resp *Response, color bool) error {
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
		return highlight(w, HTTPPreambleLexer, []byte(preamble.String()))
	}

	_, err := io.WriteString(w, preamble.String())
	return err
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

// HighlightWithLexer renders data with the provided chroma lexer and the
// shared Restish terminal style. It falls back to the original data if
// highlighting fails.
func HighlightWithLexer(lexer chroma.Lexer, data []byte) ([]byte, error) {
	var buf strings.Builder
	if err := highlight(&buf, lexer, data); err != nil {
		return data, err
	}
	return []byte(buf.String()), nil
}

var displayRanges = []*unicode.RangeTable{
	unicode.L, unicode.M, unicode.N, unicode.P, unicode.S, unicode.White_Space,
}

func printableBody(resp *Response) ([]byte, bool) {
	if resp == nil {
		return nil, false
	}
	switch body := resp.Body.(type) {
	case string:
		if len(resp.Raw) > 0 && isPrintableText(resp.Raw) {
			return resp.Raw, true
		}
		if isPrintableText([]byte(body)) {
			return []byte(body), true
		}
	case []byte:
		if isPrintableText(body) {
			return body, true
		}
	}
	return nil, false
}

func isPrintableText(data []byte) bool {
	if len(data) >= 102400 || !utf8.Valid(data) {
		return false
	}
	for i, r := range string(data) {
		if i == 0 && r == '\uFEFF' {
			continue
		}
		if i > 100 {
			break
		}
		if !unicode.In(r, displayRanges...) {
			return false
		}
	}
	return true
}
