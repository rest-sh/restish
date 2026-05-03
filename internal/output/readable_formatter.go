package output

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/rest-sh/restish/v2/internal/content"
	"github.com/rest-sh/restish/v2/internal/secrets"
	"golang.org/x/term"
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
	if strings.HasPrefix(Header(resp.Headers, "Content-Type"), "image/") && len(resp.Raw) > 0 {
		return (&ImageFormatter{}).Format(w, resp, color)
	}

	if data, ok := printableBody(resp); ok {
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		if color {
			if markdownBody(resp) {
				rendered, err := renderMarkdownBody(w, string(data))
				if err == nil {
					_, err = io.WriteString(w, rendered)
					return err
				}
			}
			if lexer := textBodyLexer(resp); lexer != nil {
				return highlight(w, lexer, data)
			}
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
		if _, err := io.WriteString(s.w, s.frame.Prefix); err != nil {
			return err
		}
		if err := s.writeFrameBracket("["); err != nil {
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
		return highlightReadableWithDepth(s.w, item, s.itemDepth())
	}
	_, err = s.w.Write(item)
	return err
}

func (s *readableFramedValueStream) Close() error {
	if !s.started {
		if _, err := io.WriteString(s.w, s.frame.Prefix); err != nil {
			return err
		}
		if err := s.writeFrameBracket("["); err != nil {
			return err
		}
		if err := s.writeFrameBracket("]"); err != nil {
			return err
		}
		if _, err := io.WriteString(s.w, s.frame.Suffix+"\n"); err != nil {
			return err
		}
		return nil
	}

	if s.wroteItem {
		if _, err := io.WriteString(s.w, "\n"+s.frame.CloseIndent); err != nil {
			return err
		}
		if err := s.writeFrameBracket("]"); err != nil {
			return err
		}
		if _, err := io.WriteString(s.w, s.frame.Suffix+"\n"); err != nil {
			return err
		}
		return nil
	}

	if err := s.writeFrameBracket("]"); err != nil {
		return err
	}
	_, err := io.WriteString(s.w, s.frame.Suffix+"\n")
	return err
}

func (s *readableFramedValueStream) writeFrameBracket(bracket string) error {
	if !s.color {
		_, err := io.WriteString(s.w, bracket)
		return err
	}
	return highlightToken(s.w, chroma.Token{
		Type:  shiftedIndentToken(indentLevel0, s.arrayDepth()),
		Value: bracket,
	})
}

func (s *readableFramedValueStream) arrayDepth() int {
	return indentDepth(s.frame.CloseIndent)
}

func (s *readableFramedValueStream) itemDepth() int {
	return s.arrayDepth() + 1
}

func indentDepth(indent string) int {
	spaces := 0
	for _, r := range indent {
		switch r {
		case ' ':
			spaces++
		case '\t':
			spaces += 2
		}
	}
	return spaces / 2
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
		for _, value := range resp.Headers[k] {
			if secrets.IsHeaderName(k) {
				value = "<redacted>"
			}
			fmt.Fprintf(&preamble, "%s: %s\n", k, value)
		}
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

func highlightReadableWithDepth(w io.Writer, data []byte, depth int) error {
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		_, err := w.Write(data)
		return err
	}

	iter, err := ReadableLexer.Tokenise(nil, string(data))
	if err != nil {
		_, werr := w.Write(data)
		return werr
	}

	return formatter.Format(w, restishStyle, indentShiftIterator(iter, depth))
}

func indentShiftIterator(iter chroma.Iterator, depth int) chroma.Iterator {
	return func() chroma.Token {
		tok := iter()
		if tok == chroma.EOF {
			return tok
		}
		tok.Type = shiftedIndentToken(tok.Type, depth)
		return tok
	}
}

func shiftedIndentToken(tok chroma.TokenType, depth int) chroma.TokenType {
	if tok < indentLevel0 || tok > indentLevel2 {
		return tok
	}
	offset := int(tok - indentLevel0)
	return indentLevel0 + chroma.TokenType((offset+depth)%3)
}

func highlightToken(w io.Writer, token chroma.Token) error {
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		_, err := io.WriteString(w, token.Value)
		return err
	}
	return formatter.Format(w, restishStyle, chroma.Literator(token))
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

func textBodyLexer(resp *Response) chroma.Lexer {
	if resp == nil {
		return nil
	}
	if lexer := textBodyLexerByContentType(Header(resp.Headers, "Content-Type")); lexer != nil {
		return lexer
	}
	if resp.URL == "" {
		return nil
	}
	name := resp.URL
	if u, err := url.Parse(resp.URL); err == nil && u.Path != "" {
		name = u.Path
	}
	if lexer := lexers.Match(name); highlightableLexer(lexer) {
		return chroma.Coalesce(lexer)
	}
	return nil
}

func markdownBody(resp *Response) bool {
	if resp == nil {
		return false
	}
	contentType := Header(resp.Headers, "Content-Type")
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "text/markdown", "text/x-markdown":
		return true
	}
	if resp.URL == "" {
		return false
	}
	name := resp.URL
	if u, err := url.Parse(resp.URL); err == nil && u.Path != "" {
		name = u.Path
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	default:
		return false
	}
}

func renderMarkdownBody(w io.Writer, s string) (string, error) {
	width := 80
	if f, ok := w.(interface{ Fd() uintptr }); ok {
		if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
			width = cols
		}
	}
	r, err := NewMarkdownRenderer(width)
	if err != nil {
		return "", err
	}
	return r.Render(s)
}

func genericTextContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "text/plain", "application/octet-stream":
		return true
	default:
		return false
	}
}

func highlightableLexer(lexer chroma.Lexer) bool {
	if lexer == nil || lexer == lexers.Fallback {
		return false
	}
	cfg := lexer.Config()
	if cfg == nil {
		return false
	}
	name := strings.ToLower(cfg.Name)
	if name == "plaintext" || name == "plain text" {
		return false
	}
	for _, alias := range cfg.Aliases {
		switch strings.ToLower(alias) {
		case "text", "plain", "plaintext":
			return false
		}
	}
	return true
}

func textBodyLexerByContentType(contentType string) chroma.Lexer {
	if contentType == "" {
		return nil
	}
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}
	if !genericTextContentType(contentType) {
		if lexer := lexers.MatchMimeType(contentType); highlightableLexer(lexer) {
			return chroma.Coalesce(lexer)
		}
	}
	return nil
}

func printableBody(resp *Response) ([]byte, bool) {
	if resp == nil {
		return nil, false
	}
	switch body := resp.Body.(type) {
	case string:
		if len(resp.Raw) > 0 {
			if data, ok := content.Printable(resp.Raw); ok {
				return data, true
			}
		}
		return content.Printable([]byte(body))
	case []byte:
		return content.Printable(body)
	}
	return nil, false
}
