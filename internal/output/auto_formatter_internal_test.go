package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestIndentBlockPreservesLines(t *testing.T) {
	got := indentBlock([]byte("{\n  \"a\": 1\n}"), "  ")
	want := []byte("  {\n    \"a\": 1\n  }")
	if !bytes.Equal(got, want) {
		t.Fatalf("indentBlock() = %q, want %q", got, want)
	}
}

func TestAutoFramedValueStreamHighlightsFrameAndNestedItemDepth(t *testing.T) {
	var out bytes.Buffer
	stream, err := (&AutoFormatter{}).StartFramedValueStream(&out, nil, true, FramedValueTemplate{
		ItemIndent:  "  ",
		CloseIndent: "",
	})
	if err != nil {
		t.Fatalf("StartFramedValueStream: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 1}); err != nil {
		t.Fatalf("WriteValue: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var wantOpenArray bytes.Buffer
	if err := highlightToken(&wantOpenArray, chroma.Token{Type: indentLevel0, Value: "["}); err != nil {
		t.Fatalf("highlightToken([): %v", err)
	}
	var wantItemObject bytes.Buffer
	if err := highlightToken(&wantItemObject, chroma.Token{Type: indentLevel1, Value: "{"}); err != nil {
		t.Fatalf("highlightToken({): %v", err)
	}
	var wantCloseArray bytes.Buffer
	if err := highlightToken(&wantCloseArray, chroma.Token{Type: indentLevel0, Value: "]"}); err != nil {
		t.Fatalf("highlightToken(]): %v", err)
	}

	got := out.String()
	for _, want := range []string{wantOpenArray.String(), wantItemObject.String(), wantCloseArray.String()} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected highlighted token %q in output %q", want, got)
		}
	}
	if plain := stripANSITest(got); strings.TrimSpace(plain) != "[\n  {\n    \"id\": 1\n  }\n]" {
		t.Fatalf("unexpected plain framed output: %q", plain)
	}
}

func TestAutoFramedValueStreamHighlightsWrappedArrayDepth(t *testing.T) {
	var out bytes.Buffer
	stream, err := (&AutoFormatter{}).StartFramedValueStream(&out, nil, true, FramedValueTemplate{
		Prefix:      "{\n  \"data\": ",
		Suffix:      "\n}",
		ItemIndent:  "    ",
		CloseIndent: "  ",
	})
	if err != nil {
		t.Fatalf("StartFramedValueStream: %v", err)
	}
	if err := stream.WriteValue(map[string]any{"id": 1}); err != nil {
		t.Fatalf("WriteValue: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var wantArray bytes.Buffer
	if err := highlightToken(&wantArray, chroma.Token{Type: indentLevel1, Value: "["}); err != nil {
		t.Fatalf("highlightToken([): %v", err)
	}
	var wantItemObject bytes.Buffer
	if err := highlightToken(&wantItemObject, chroma.Token{Type: indentLevel2, Value: "{"}); err != nil {
		t.Fatalf("highlightToken({): %v", err)
	}

	got := out.String()
	for _, want := range []string{wantArray.String(), wantItemObject.String()} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected highlighted token %q in output %q", want, got)
		}
	}
}

func TestShiftedIndentTokenCyclesBracketDepth(t *testing.T) {
	if got := shiftedIndentToken(indentLevel0, 1); got != indentLevel1 {
		t.Fatalf("shiftedIndentToken(0, 1) = %v, want %v", got, indentLevel1)
	}
	if got := shiftedIndentToken(indentLevel2, 1); got != indentLevel0 {
		t.Fatalf("shiftedIndentToken(2, 1) = %v, want %v", got, indentLevel0)
	}
	if got := shiftedIndentToken(chroma.NameTag, 1); got != chroma.NameTag {
		t.Fatalf("shiftedIndentToken(non-indent) = %v, want %v", got, chroma.NameTag)
	}
}

func TestAutoFormatterHighlightsPrintableTextByURLPath(t *testing.T) {
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}},
		URL:     "https://raw.githubusercontent.com/rest-sh/restish/main/cmd/restish/main.go?token=secret",
		Body:    "package main\n\nfunc main() {}\n",
		Raw:     []byte("package main\n\nfunc main() {}\n"),
	}
	if got := textBodyLexer(resp).Config().Name; got != "Go" {
		t.Fatalf("textBodyLexer() = %q, want Go", got)
	}

	var out bytes.Buffer
	if err := (&AutoFormatter{}).Format(&out, resp, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI-highlighted output, got %q", got)
	}
	plain := stripANSITest(got)
	if !strings.Contains(plain, "package main") || !strings.Contains(plain, "func main") {
		t.Fatalf("expected Go source in output, got %q", got)
	}
}

func TestAutoFormatterHighlightsPrintableTextByContentType(t *testing.T) {
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"text/html; charset=utf-8"}},
		Body:    "<!doctype html>\n<title>Hello</title>\n",
		Raw:     []byte("<!doctype html>\n<title>Hello</title>\n"),
	}
	if got := textBodyLexer(resp).Config().Name; got != "HTML" {
		t.Fatalf("textBodyLexer() = %q, want HTML", got)
	}

	var out bytes.Buffer
	if err := (&AutoFormatter{}).Format(&out, resp, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI-highlighted output, got %q", got)
	}
}

func TestAutoFormatterRendersLargeDeclaredTextAsText(t *testing.T) {
	css := strings.Repeat(".restish{color:#2dd4bf}", 5000)
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"text/css; charset=utf-8"}},
		Body:    css,
		Raw:     []byte(css),
	}
	if got := textBodyLexer(resp).Config().Name; got != "CSS" {
		t.Fatalf("textBodyLexer() = %q, want CSS", got)
	}

	var out bytes.Buffer
	if err := (&AutoFormatter{}).Format(&out, resp, false); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	body := strings.TrimSpace(out.String())
	if strings.HasPrefix(body, `"`) || strings.Contains(body, `\"`) {
		t.Fatalf("expected raw CSS text, got JSON string body starting %q", body[:80])
	}
	if !strings.Contains(body, ".restish{color:#2dd4bf}") {
		t.Fatalf("expected CSS body, got %q", body[:80])
	}
}

func TestAutoFormatterRendersLargeLexerMatchedBytesAsText(t *testing.T) {
	js := strings.Repeat("const answer = 42;\n", 7000)
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/javascript; charset=utf-8"}},
		Body:    []byte(js),
		Raw:     []byte(js),
	}
	if got := textBodyLexer(resp).Config().Name; got != "JavaScript" {
		t.Fatalf("textBodyLexer() = %q, want JavaScript", got)
	}

	var out bytes.Buffer
	if err := (&AutoFormatter{}).Format(&out, resp, false); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	body := strings.TrimSpace(out.String())
	if strings.HasPrefix(body, `"`) || !strings.Contains(body, "const answer = 42;") {
		t.Fatalf("expected raw JavaScript text, got body starting %q", body[:80])
	}
}

func TestAutoFormatterRendersMarkdownByURLPath(t *testing.T) {
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"text/plain; charset=utf-8"}},
		URL:     "https://raw.githubusercontent.com/rest-sh/restish/main/README.md?token=secret",
		Body:    "# Restish\n\n- Talk to APIs\n",
		Raw:     []byte("# Restish\n\n- Talk to APIs\n"),
	}
	if !markdownBody(resp) {
		t.Fatal("expected markdownBody to detect .md response URL")
	}

	var out bytes.Buffer
	if err := (&AutoFormatter{}).Format(&out, resp, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	plain := stripANSITest(out.String())
	if strings.Contains(plain, "- Talk to APIs") {
		t.Fatalf("expected rendered Markdown, got raw list marker in %q", out.String())
	}
	if !strings.Contains(plain, "Restish") || !strings.Contains(plain, "Talk to APIs") {
		t.Fatalf("expected Markdown content in output, got %q", out.String())
	}
}

func TestAutoFormatterRendersMarkdownWithRestishTheme(t *testing.T) {
	t.Setenv("GLAMOUR_STYLE", "")
	if err := SetTheme(ThemeEntries{"markdown_heading": "#123456"}); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	defer func() {
		if err := SetTheme(nil); err != nil {
			t.Fatalf("reset theme: %v", err)
		}
	}()

	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"text/markdown; charset=utf-8"}},
		Body:    "## Restish\n",
		Raw:     []byte("## Restish\n"),
	}
	var out bytes.Buffer
	if err := (&AutoFormatter{}).Format(&out, resp, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "\x1b[38;2;18;52;86") {
		t.Fatalf("expected themed Markdown heading color, got %q", got)
	}
}

func TestMarkdownRendererHighlightsSchemaFencesWithRestishTheme(t *testing.T) {
	t.Setenv("GLAMOUR_STYLE", "")
	if err := SetTheme(ThemeEntries{"key": "#ff0000"}); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	defer func() {
		if err := SetTheme(nil); err != nil {
			t.Fatalf("reset theme: %v", err)
		}
	}()

	renderer, err := NewMarkdownRenderer(80)
	if err != nil {
		t.Fatalf("NewMarkdownRenderer: %v", err)
	}
	got, err := renderer.Render("```schema\n{\n  name*: (string)\n}\n```\n")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "\x1b[38;5;196mname") && !strings.Contains(got, "\x1b[38;2;255;0;0mname") {
		t.Fatalf("expected themed schema key color, got %q", got)
	}
	if plain := stripANSITest(got); !strings.Contains(plain, "name*: (string)") {
		t.Fatalf("expected schema text after rendering, got %q", got)
	}
}

func TestSchemaLexerHighlightsUsefulSchemaParts(t *testing.T) {
	iter, err := SchemaLexer.Tokenise(nil, "{\n  name*: (string format:uri default:about:blank)\n}")
	if err != nil {
		t.Fatalf("Tokenise: %v", err)
	}

	tokens := iter.Tokens()
	for _, want := range []chroma.Token{
		{Type: chroma.NameTag, Value: "name"},
		{Type: chroma.Operator, Value: "*"},
		{Type: chroma.KeywordType, Value: "string"},
		{Type: chroma.NameBuiltin, Value: "format:"},
		{Type: chroma.LiteralString, Value: "uri"},
		{Type: chroma.NameBuiltin, Value: "default:"},
		{Type: chroma.LiteralString, Value: "about:blank"},
	} {
		if !tokenSliceContains(tokens, want) {
			t.Fatalf("expected token %s %q in %#v", want.Type, want.Value, tokens)
		}
	}
}

func TestReadableLexerHighlightsHTTPDate(t *testing.T) {
	iter, err := ReadableLexer.Tokenise(nil, `{"date":"Wed, 29 Apr 2026 05:02:53 GMT"}`)
	if err != nil {
		t.Fatalf("Tokenise: %v", err)
	}

	tokens := iter.Tokens()
	want := chroma.Token{Type: chroma.LiteralDate, Value: `"Wed, 29 Apr 2026 05:02:53 GMT"`}
	if !tokenSliceContains(tokens, want) {
		t.Fatalf("expected HTTP date token %s %q in %#v", want.Type, want.Value, tokens)
	}
}

func TestHTTPPreambleLexerLeavesHeaderValuesPlain(t *testing.T) {
	iter, err := HTTPPreambleLexer.Tokenise(nil, "HTTP/1.1 200 OK\nDate: Wed, 29 Apr 2026 05:02:53 GMT\n")
	if err != nil {
		t.Fatalf("Tokenise: %v", err)
	}

	tokens := iter.Tokens()
	want := chroma.Token{Type: chroma.Text, Value: "Wed, 29 Apr 2026 05:02:53 GMT"}
	if !tokenSliceContains(tokens, want) {
		t.Fatalf("expected plain HTTP header value token %s %q in %#v", want.Type, want.Value, tokens)
	}
	unwanted := chroma.Token{Type: chroma.LiteralDate, Value: "Wed, 29 Apr 2026 05:02:53 GMT"}
	if tokenSliceContains(tokens, unwanted) {
		t.Fatalf("did not expect highlighted HTTP date header token %s %q in %#v", unwanted.Type, unwanted.Value, tokens)
	}
}

func TestHTTPPreambleLexerDoesNotRetokenizeHeaderValueColons(t *testing.T) {
	iter, err := HTTPPreambleLexer.Tokenise(nil, "HTTP/1.1 200 OK\nX-Request-Time: 2026-05-14T21:51:04Z\nLocation: https://api.example.test/items/42\n")
	if err != nil {
		t.Fatalf("Tokenise: %v", err)
	}

	tokens := iter.Tokens()
	for _, unwanted := range []chroma.Token{
		{Type: httpHeaderKey, Value: "2026-05-14T21"},
		{Type: httpHeaderKey, Value: "https"},
	} {
		if tokenSliceContains(tokens, unwanted) {
			t.Fatalf("did not expect header value token %s %q in %#v", unwanted.Type, unwanted.Value, tokens)
		}
	}
	for _, want := range []chroma.Token{
		{Type: chroma.Text, Value: "2026-05-14T21:51:04Z"},
		{Type: chroma.Text, Value: "https://api.example.test/items/42"},
	} {
		if !tokenSliceContains(tokens, want) {
			t.Fatalf("expected plain HTTP header value token %s %q in %#v", want.Type, want.Value, tokens)
		}
	}
}

func TestHTTPPreambleLexerUsesHeaderKeyToken(t *testing.T) {
	iter, err := HTTPPreambleLexer.Tokenise(nil, "HTTP/1.1 200 OK\nContent-Type: application/json\n")
	if err != nil {
		t.Fatalf("Tokenise: %v", err)
	}

	tokens := iter.Tokens()
	want := chroma.Token{Type: httpHeaderKey, Value: "Content-Type"}
	if !tokenSliceContains(tokens, want) {
		t.Fatalf("expected HTTP header key token %s %q in %#v", want.Type, want.Value, tokens)
	}
}

func TestAutoFormatterHonorsGlamourStyleEnv(t *testing.T) {
	t.Setenv("GLAMOUR_STYLE", "notty")
	if err := SetTheme(ThemeEntries{"markdown_heading": "#123456"}); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	defer func() {
		if err := SetTheme(nil); err != nil {
			t.Fatalf("reset theme: %v", err)
		}
	}()

	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"text/markdown; charset=utf-8"}},
		Body:    "## Restish\n",
		Raw:     []byte("## Restish\n"),
	}
	var out bytes.Buffer
	if err := (&AutoFormatter{}).Format(&out, resp, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if got := out.String(); strings.Contains(got, "\x1b[38;2;18;52;86") {
		t.Fatalf("expected GLAMOUR_STYLE to override Restish Markdown theme, got %q", got)
	}
}

func TestMarkdownBodyDetectsMarkdownContentType(t *testing.T) {
	resp := &Response{
		Headers: map[string][]string{"Content-Type": {"text/markdown; charset=utf-8"}},
		Body:    "# Restish\n",
		Raw:     []byte("# Restish\n"),
	}
	if !markdownBody(resp) {
		t.Fatal("expected markdownBody to detect text/markdown")
	}
}

func stripANSITest(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7E) {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func tokenSliceContains(tokens []chroma.Token, want chroma.Token) bool {
	for _, tok := range tokens {
		if tok.Type == want.Type && tok.Value == want.Value {
			return true
		}
	}
	return false
}
