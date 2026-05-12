package output

import (
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

var markdownStyleCache struct {
	sync.Mutex
	key   string
	style ansi.StyleConfig
	ok    bool
}

// NewMarkdownRenderer returns a Glamour renderer that respects GLAMOUR_STYLE
// when explicitly configured, otherwise falling back to the active Restish
// theme-backed Markdown style.
func NewMarkdownRenderer(width int) (*glamour.TermRenderer, error) {
	if style, ok := os.LookupEnv("GLAMOUR_STYLE"); ok && strings.TrimSpace(style) != "" {
		if r, err := glamour.NewTermRenderer(
			glamour.WithEnvironmentConfig(),
			glamour.WithWordWrap(width),
		); err == nil {
			return r, nil
		}
	}
	return glamour.NewTermRenderer(
		glamour.WithStyles(MarkdownStyle()),
		glamour.WithWordWrap(width),
	)
}

// MarkdownStyle returns the Restish Glamour style, with colors derived from the
// active user theme so Markdown bodies and help match readable output.
func MarkdownStyle() ansi.StyleConfig {
	style, entries := themeSnapshot()
	key := themeCacheKey(entries)
	markdownStyleCache.Lock()
	if markdownStyleCache.ok && markdownStyleCache.key == key {
		styleConfig := markdownStyleCache.style
		markdownStyleCache.Unlock()
		return styleConfig
	}
	markdownStyleCache.Unlock()

	styleConfig := buildMarkdownStyle(style, entries)

	markdownStyleCache.Lock()
	markdownStyleCache.key = key
	markdownStyleCache.style = styleConfig
	markdownStyleCache.ok = true
	markdownStyleCache.Unlock()
	return styleConfig
}

func resetMarkdownStyleCache() {
	markdownStyleCache.Lock()
	markdownStyleCache.ok = false
	markdownStyleCache.Unlock()
}

func themeCacheKey(entries ThemeEntries) string {
	if len(entries) == 0 {
		return ""
	}
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(entries[key])
		b.WriteByte('\n')
	}
	return b.String()
}

func buildMarkdownStyle(style *chroma.Style, entries ThemeEntries) ansi.StyleConfig {
	document := markdownPrimitive(entries, style, "markdown_document", chroma.Text)
	quote := markdownPrimitive(entries, style, "markdown_quote", chroma.GenericEmph)
	heading := markdownPrimitive(entries, style, "markdown_heading", chroma.GenericHeading)
	h1 := markdownPrimitive(entries, style, "markdown_h1", chroma.GenericHeading)
	h1Text := markdownStyleEntry(entries, "markdown_h1_text", chroma.StyleEntry{Colour: chroma.MustParseColour("#000000")})
	h1Background := markdownStyleEntry(entries, "markdown_h1_background", style.Get(chroma.KeywordConstant))
	link := markdownPrimitive(entries, style, "markdown_link", chroma.LiteralStringSymbol)
	linkText := markdownPrimitive(entries, style, "markdown_link_text", chroma.LiteralString)
	code := markdownPrimitive(entries, style, "markdown_code", chroma.LiteralNumber)
	codeBlock := markdownPrimitive(entries, style, "markdown_code_block", chroma.Text)
	codeBackground := markdownStyleEntry(entries, "markdown_code_background", chroma.StyleEntry{Background: chroma.MustParseColour("#303030")})
	rule := markdownPrimitive(entries, style, "markdown_rule", chroma.Comment)
	tableBorder := markdownPrimitive(entries, style, "markdown_table_border", chroma.Punctuation)
	image := markdownPrimitive(entries, style, "markdown_image", chroma.GenericDeleted)
	imageText := markdownPrimitive(entries, style, "markdown_image_text", chroma.Comment)

	h1.Color = colorPtr(h1Text)
	h1.BackgroundColor = backgroundOrColorPtr(h1Background)
	h1.Prefix = " "
	h1.Suffix = " "
	h1.Bold = boolPtr(true)
	code.Prefix = " "
	code.Suffix = " "
	code.BackgroundColor = backgroundPtr(codeBackground)
	if code.BackgroundColor == nil {
		code.BackgroundColor = colorPtr(codeBackground)
	}

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       document.Color,
			},
			Margin: uintPtr(2),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: quote.Color,
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       heading.Color,
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{StylePrimitive: h1},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "## "},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "### "},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "#### "},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "##### "},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  heading.Color,
				Bold:   boolPtr(false),
			},
		},
		Strikethrough: ansi.StylePrimitive{CrossedOut: boolPtr(true)},
		Emph:          ansi.StylePrimitive{Italic: boolPtr(true)},
		Strong:        ansi.StylePrimitive{Bold: boolPtr(true)},
		HorizontalRule: ansi.StylePrimitive{
			Color:  rule.Color,
			Format: "\n--------\n",
		},
		Item:        ansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: ansi.StylePrimitive{BlockPrefix: ". "},
		Task: ansi.StyleTask{
			StylePrimitive: ansi.StylePrimitive{},
			Ticked:         "[✓] ",
			Unticked:       "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     link.Color,
			Italic:    boolPtr(true),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: linkText.Color,
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     image.Color,
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  imageText.Color,
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{StylePrimitive: code},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: codeBlock.Color},
				Margin:         uintPtr(2),
			},
			Chroma: markdownChroma(style, entries),
		},
		Table: ansi.StyleTable{
			StyleBlock:      ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Color: tableBorder.Color}},
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n🠶 ",
		},
	}
}

func markdownChroma(style *chroma.Style, entries ThemeEntries) *ansi.Chroma {
	return &ansi.Chroma{
		Text:                markdownPrimitive(entries, style, "", chroma.Text),
		Error:               ansi.StylePrimitive{Color: stringPtr("#f1f1f1"), BackgroundColor: stringPtr("#f05b5b")},
		Comment:             markdownPrimitive(entries, style, "", chroma.Comment),
		CommentPreproc:      markdownPrimitive(entries, style, "", chroma.CommentPreproc),
		Keyword:             markdownPrimitive(entries, style, "", chroma.Keyword),
		KeywordReserved:     markdownPrimitive(entries, style, "", chroma.KeywordReserved),
		KeywordNamespace:    markdownPrimitive(entries, style, "", chroma.KeywordNamespace),
		KeywordType:         markdownPrimitive(entries, style, "", chroma.KeywordType),
		Operator:            markdownPrimitive(entries, style, "", chroma.Operator),
		Punctuation:         markdownPrimitive(entries, style, "", chroma.Punctuation),
		Name:                markdownPrimitive(entries, style, "", chroma.Name),
		NameBuiltin:         markdownPrimitive(entries, style, "", chroma.NameBuiltin),
		NameTag:             markdownPrimitive(entries, style, "", chroma.NameTag),
		NameAttribute:       markdownPrimitive(entries, style, "", chroma.NameAttribute),
		NameClass:           markdownPrimitive(entries, style, "", chroma.NameClass),
		NameDecorator:       markdownPrimitive(entries, style, "", chroma.NameDecorator),
		NameFunction:        markdownPrimitive(entries, style, "", chroma.NameFunction),
		LiteralNumber:       markdownPrimitive(entries, style, "", chroma.LiteralNumber),
		LiteralString:       markdownPrimitive(entries, style, "", chroma.LiteralString),
		LiteralStringEscape: markdownPrimitive(entries, style, "", chroma.LiteralStringEscape),
		GenericDeleted:      markdownPrimitive(entries, style, "", chroma.GenericDeleted),
		GenericEmph:         markdownPrimitive(entries, style, "", chroma.GenericEmph),
		GenericInserted:     markdownPrimitive(entries, style, "", chroma.GenericInserted),
		GenericStrong:       markdownPrimitive(entries, style, "", chroma.GenericStrong),
		GenericSubheading:   markdownPrimitive(entries, style, "", chroma.GenericSubheading),
		Background:          ansi.StylePrimitive{BackgroundColor: stringPtr("#373737")},
	}
}

func markdownPrimitive(entries ThemeEntries, style *chroma.Style, alias string, token chroma.TokenType) ansi.StylePrimitive {
	return primitiveFromStyleEntry(markdownStyleEntry(entries, alias, style.Get(token)))
}

func markdownStyleEntry(entries ThemeEntries, alias string, fallback chroma.StyleEntry) chroma.StyleEntry {
	if alias != "" {
		if entry, ok := entries[alias]; ok {
			if parsed, err := chroma.ParseStyleEntry(entry); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func primitiveFromStyleEntry(entry chroma.StyleEntry) ansi.StylePrimitive {
	return ansi.StylePrimitive{
		Color:           colorPtr(entry),
		BackgroundColor: backgroundPtr(entry),
		Bold:            trileanPtr(entry.Bold),
		Italic:          trileanPtr(entry.Italic),
		Underline:       trileanPtr(entry.Underline),
	}
}

func colorPtr(entry chroma.StyleEntry) *string {
	if !entry.Colour.IsSet() {
		return nil
	}
	return stringPtr(entry.Colour.String())
}

func backgroundPtr(entry chroma.StyleEntry) *string {
	if !entry.Background.IsSet() {
		return nil
	}
	return stringPtr(entry.Background.String())
}

func backgroundOrColorPtr(entry chroma.StyleEntry) *string {
	if background := backgroundPtr(entry); background != nil {
		return background
	}
	return colorPtr(entry)
}

func trileanPtr(v chroma.Trilean) *bool {
	switch v {
	case chroma.Yes:
		return boolPtr(true)
	case chroma.No:
		return boolPtr(false)
	default:
		return nil
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

func uintPtr(v uint) *uint {
	return &v
}
