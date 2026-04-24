package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestIndentBlockPreservesLines(t *testing.T) {
	got := indentBlock([]byte("{\n  \"a\": 1\n}"), "  ")
	want := []byte("  {\n    \"a\": 1\n  }")
	if !bytes.Equal(got, want) {
		t.Fatalf("indentBlock() = %q, want %q", got, want)
	}
}

func TestReadableFormatterHighlightsPrintableTextByURLPath(t *testing.T) {
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"},
		URL:     "https://raw.githubusercontent.com/rest-sh/restish/main/cmd/restish/main.go?token=secret",
		Body:    "package main\n\nfunc main() {}\n",
		Raw:     []byte("package main\n\nfunc main() {}\n"),
	}
	if got := textBodyLexer(resp).Config().Name; got != "Go" {
		t.Fatalf("textBodyLexer() = %q, want Go", got)
	}

	var out bytes.Buffer
	if err := (&ReadableFormatter{}).Format(&out, resp, true); err != nil {
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

func TestReadableFormatterHighlightsPrintableTextByContentType(t *testing.T) {
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string]string{"Content-Type": "text/html; charset=utf-8"},
		Body:    "<!doctype html>\n<title>Hello</title>\n",
		Raw:     []byte("<!doctype html>\n<title>Hello</title>\n"),
	}
	if got := textBodyLexer(resp).Config().Name; got != "HTML" {
		t.Fatalf("textBodyLexer() = %q, want HTML", got)
	}

	var out bytes.Buffer
	if err := (&ReadableFormatter{}).Format(&out, resp, true); err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI-highlighted output, got %q", got)
	}
}

func TestReadableFormatterRendersMarkdownByURLPath(t *testing.T) {
	resp := &Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string]string{"Content-Type": "text/plain; charset=utf-8"},
		URL:     "https://raw.githubusercontent.com/rest-sh/restish/main/README.md?token=secret",
		Body:    "# Restish\n\n- Talk to APIs\n",
		Raw:     []byte("# Restish\n\n- Talk to APIs\n"),
	}
	if !markdownBody(resp) {
		t.Fatal("expected markdownBody to detect .md response URL")
	}

	var out bytes.Buffer
	if err := (&ReadableFormatter{}).Format(&out, resp, true); err != nil {
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

func TestMarkdownBodyDetectsMarkdownContentType(t *testing.T) {
	resp := &Response{
		Headers: map[string]string{"Content-Type": "text/markdown; charset=utf-8"},
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
