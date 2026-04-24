package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// ThemeEntries maps token names to Chroma style descriptors. Token names may be
// Chroma token type names (for example, "NameTag") or Restish aliases such as
// "key", "url", and "status_2xx".
type ThemeEntries map[string]string

// defaultStyleEntries is a 256-color terminal theme for Restish readable
// output. It assigns colors to all token types produced by ReadableLexer as
// well as standard JSON token types.
var defaultStyleEntries = chroma.StyleEntries{
	// Core JSON token types
	chroma.Comment:              "#9e9e9e",
	chroma.KeywordConstant:      "#ff5f87", // true / false / null
	chroma.Punctuation:          "#9e9e9e", // : , braces handled by indent tokens
	chroma.NameTag:              "#5fafd7", // object keys
	chroma.LiteralNumber:        "#d78700",
	chroma.LiteralNumberFloat:   "#d78700",
	chroma.LiteralNumberInteger: "#d78700",
	chroma.LiteralString:        "#afd787",
	chroma.LiteralStringDouble:  "#afd787",

	// Special value types detected by ReadableLexer
	chroma.LiteralStringSymbol: "italic #D6FFB7", // URLs
	chroma.LiteralDate:         "#af87af",        // ISO 8601 dates
	chroma.LiteralNumberHex:    "#ffd7d7",        // hex binary (0x...)

	// Bracket-depth colorization: IndentLevel0/1/2 cycle through three colours.
	IndentLevel0: "#d78700", // amber
	IndentLevel1: "#af87af", // mauve
	IndentLevel2: "#5fafd7", // sky blue

	// HTTP preamble (status line + headers via HTTPPreambleLexer)
	chroma.NameNamespace:   "#9e9e9e",      // HTTP/x.x  → gray
	chroma.GenericInserted: "bold #afd787", // 2xx       → bold light-green
	chroma.GenericOutput:   "bold #d78700", // 3xx       → bold amber
	chroma.GenericError:    "bold #ff5f87", // 4xx/5xx   → bold pink
}

// restishStyle is the active style for terminal highlighting.
var restishStyle = styles.Register(chroma.MustNewStyle("restish", defaultStyleEntries))

var themeTokenAliases = map[string]chroma.TokenType{
	"comment":       chroma.Comment,
	"constant":      chroma.KeywordConstant,
	"punctuation":   chroma.Punctuation,
	"key":           chroma.NameTag,
	"number":        chroma.LiteralNumber,
	"float":         chroma.LiteralNumberFloat,
	"integer":       chroma.LiteralNumberInteger,
	"string":        chroma.LiteralString,
	"quoted_string": chroma.LiteralStringDouble,
	"url":           chroma.LiteralStringSymbol,
	"date":          chroma.LiteralDate,
	"binary":        chroma.LiteralNumberHex,
	"bracket_0":     IndentLevel0,
	"bracket_1":     IndentLevel1,
	"bracket_2":     IndentLevel2,
	"http":          chroma.NameNamespace,
	"status_2xx":    chroma.GenericInserted,
	"status_3xx":    chroma.GenericOutput,
	"status_error":  chroma.GenericError,
}

// SetTheme overlays user-supplied theme entries onto the built-in Restish
// theme. Passing nil or an empty map restores the default style.
func SetTheme(entries ThemeEntries) error {
	style, err := BuildTheme(entries)
	if err != nil {
		return err
	}
	restishStyle = style
	return nil
}

// BuildTheme validates user theme entries and returns a Chroma style.
func BuildTheme(entries ThemeEntries) (*chroma.Style, error) {
	styleEntries := make(chroma.StyleEntries, len(defaultStyleEntries)+len(entries))
	for token, entry := range defaultStyleEntries {
		styleEntries[token] = entry
	}
	for name, entry := range entries {
		token, err := themeTokenType(name)
		if err != nil {
			return nil, err
		}
		styleEntries[token] = entry
	}
	style, err := chroma.NewStyle("restish", styleEntries)
	if err != nil {
		return nil, fmt.Errorf("theme: %w", err)
	}
	return style, nil
}

// ParseThemeJSON parses a direct token map and validates it by building a style.
func ParseThemeJSON(data []byte) (ThemeEntries, error) {
	var direct ThemeEntries
	if err := json.Unmarshal(data, &direct); err != nil {
		return nil, fmt.Errorf("theme: parse JSON: %w", err)
	}
	if len(direct) == 0 {
		return nil, fmt.Errorf("theme: expected token map")
	}
	if _, err := BuildTheme(direct); err != nil {
		return nil, err
	}
	return direct, nil
}

func themeTokenType(name string) (chroma.TokenType, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	key = strings.ReplaceAll(key, "-", "_")
	if token, ok := themeTokenAliases[key]; ok {
		return token, nil
	}
	token, err := chroma.TokenTypeString(strings.TrimSpace(name))
	if err != nil {
		return 0, fmt.Errorf("theme: unknown token %q", name)
	}
	return token, nil
}
