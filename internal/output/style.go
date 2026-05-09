package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/tidwall/jsonc"
)

// ThemeEntries maps token names to Chroma style descriptors. Token names may be
// Chroma token type names (for example, "NameTag") or Restish aliases such as
// "key", "header_key", "keyword", "url", and "status_2xx".
type ThemeEntries map[string]string

// defaultStyleEntries is a 256-color terminal theme for Restish readable
// output. It assigns colors to all token types produced by ReadableLexer as
// well as standard JSON token types.
var defaultStyleEntries = chroma.StyleEntries{
	// Core JSON token types
	chroma.Comment:              "#94a3b8",
	chroma.KeywordConstant:      "#f43f5e", // true / false / null
	chroma.Punctuation:          "#94a3b8", // : , braces handled by indent tokens
	chroma.NameTag:              "#5fafd7", // object keys
	chroma.LiteralNumber:        "#d78700",
	chroma.LiteralNumberFloat:   "#d78700",
	chroma.LiteralNumberInteger: "#d78700",
	chroma.LiteralString:        "#afd787",
	chroma.LiteralStringDouble:  "#afd787",

	// Special value types detected by ReadableLexer
	chroma.LiteralStringSymbol: "italic #d6ffb7", // URLs
	chroma.LiteralDate:         "#af87d7",        // ISO 8601 dates
	chroma.LiteralNumberHex:    "#d7afc7",        // hex binary (0x...)
	chroma.Keyword:             "#f43f5e",
	chroma.KeywordType:         "bold #f43f5e",
	chroma.NameFunction:        "#5fafd7",
	chroma.NameClass:           "#5fafd7",
	chroma.NameBuiltin:         "#87afd7",
	chroma.Operator:            "#d7af5f",

	// Bracket-depth colorization: indentLevel0/1/2 cycle through three colours.
	indentLevel0: "#d78700", // amber
	indentLevel1: "#af87d7", // violet
	indentLevel2: "#6fbfbf", // teal

	// HTTP preamble (status line + headers via HTTPPreambleLexer)
	chroma.NameNamespace:   "#94a3b8",      // HTTP/x.x  → muted slate
	httpHeaderKey:          "#6fbfbf",      // response header names
	chroma.GenericInserted: "bold #afd787", // 2xx       → bold light-green
	chroma.GenericOutput:   "bold #d78700", // 3xx       → bold amber
	chroma.GenericError:    "bold #f43f5e", // 4xx/5xx   → bold rose

	// Markdown and diff tokens used by Glamour-rendered Markdown bodies/help.
	chroma.GenericHeading:    "#5fafd7",
	chroma.GenericSubheading: "#94a3b8",
	chroma.GenericEmph:       "italic #d7afc7",
	chroma.GenericStrong:     "bold #af87d7",
	chroma.GenericDeleted:    "#f43f5e",
	chroma.NameAttribute:     "underline #6fbfbf",

	// Diagnostic labels written to stderr.
	diagnosticInfo:  "#6fbfbf",
	diagnosticWarn:  "bold #d78700",
	diagnosticError: "bold #f43f5e",
	diagnosticHint:  "#d7af5f",
}

// restishStyle is the active style for terminal highlighting.
var restishStyle = styles.Register(chroma.MustNewStyle("restish", defaultStyleEntries))
var currentThemeEntries ThemeEntries

var themeTokenAliases = map[string]chroma.TokenType{
	"comment":          chroma.Comment,
	"text":             chroma.Text,
	"constant":         chroma.KeywordConstant,
	"punctuation":      chroma.Punctuation,
	"key":              chroma.NameTag,
	"number":           chroma.LiteralNumber,
	"float":            chroma.LiteralNumberFloat,
	"integer":          chroma.LiteralNumberInteger,
	"string":           chroma.LiteralString,
	"quoted_string":    chroma.LiteralStringDouble,
	"url":              chroma.LiteralStringSymbol,
	"date":             chroma.LiteralDate,
	"binary":           chroma.LiteralNumberHex,
	"keyword":          chroma.Keyword,
	"type":             chroma.KeywordType,
	"function":         chroma.NameFunction,
	"class":            chroma.NameClass,
	"builtin":          chroma.NameBuiltin,
	"operator":         chroma.Operator,
	"bracket_0":        indentLevel0,
	"bracket_1":        indentLevel1,
	"bracket_2":        indentLevel2,
	"http":             chroma.NameNamespace,
	"header":           httpHeaderKey,
	"header_key":       httpHeaderKey,
	"status_2xx":       chroma.GenericInserted,
	"status_3xx":       chroma.GenericOutput,
	"status_error":     chroma.GenericError,
	"heading":          chroma.GenericHeading,
	"subheading":       chroma.GenericSubheading,
	"emphasis":         chroma.GenericEmph,
	"strong":           chroma.GenericStrong,
	"deleted":          chroma.GenericDeleted,
	"inserted":         chroma.GenericInserted,
	"attribute":        chroma.NameAttribute,
	"diagnostic_info":  diagnosticInfo,
	"diagnostic_warn":  diagnosticWarn,
	"diagnostic_error": diagnosticError,
	"diagnostic_hint":  diagnosticHint,
}

var markdownThemeAliases = map[string]struct{}{
	"markdown_document":        {},
	"markdown_quote":           {},
	"markdown_heading":         {},
	"markdown_h1":              {},
	"markdown_h1_text":         {},
	"markdown_h1_background":   {},
	"markdown_link":            {},
	"markdown_link_text":       {},
	"markdown_code":            {},
	"markdown_code_block":      {},
	"markdown_code_background": {},
	"markdown_rule":            {},
	"markdown_table_border":    {},
	"markdown_image":           {},
	"markdown_image_text":      {},
}

// SetTheme overlays user-supplied theme entries onto the built-in Restish
// theme. Passing nil or an empty map restores the default style.
func SetTheme(entries ThemeEntries) error {
	style, err := BuildTheme(entries)
	if err != nil {
		return err
	}
	restishStyle = style
	currentThemeEntries = normalizeThemeEntries(entries)
	resetMarkdownStyleCache()
	return nil
}

// BuildTheme validates user theme entries and returns a Chroma style.
func BuildTheme(entries ThemeEntries) (*chroma.Style, error) {
	styleEntries := make(chroma.StyleEntries, len(defaultStyleEntries)+len(entries))
	for token, entry := range defaultStyleEntries {
		styleEntries[token] = entry
	}
	keyOverridden := false
	headerKeyOverridden := false
	for name, entry := range entries {
		if _, ok := markdownThemeAliases[normalizeThemeName(name)]; ok {
			if _, err := chroma.ParseStyleEntry(entry); err != nil {
				return nil, fmt.Errorf("theme: %s: %w", name, err)
			}
			continue
		}
		token, err := themeTokenType(name)
		if err != nil {
			return nil, err
		}
		if token == chroma.NameTag {
			keyOverridden = true
		}
		if token == httpHeaderKey {
			headerKeyOverridden = true
		}
		styleEntries[token] = entry
	}
	if keyOverridden && !headerKeyOverridden {
		styleEntries[httpHeaderKey] = styleEntries[chroma.NameTag]
	}
	style, err := chroma.NewStyle("restish", styleEntries)
	if err != nil {
		return nil, fmt.Errorf("theme: %w", err)
	}
	return style, nil
}

// ParseThemeJSON parses a direct JSON or JSONC token map and validates it by
// building a style.
func ParseThemeJSON(data []byte) (ThemeEntries, error) {
	var direct ThemeEntries
	if err := json.Unmarshal(jsonc.ToJSON(data), &direct); err != nil {
		return nil, fmt.Errorf("theme: parse JSONC: %w", err)
	}
	if len(direct) == 0 {
		return nil, fmt.Errorf("theme: expected token map")
	}
	if _, err := BuildTheme(direct); err != nil {
		return nil, err
	}
	return direct, nil
}

// StyleText renders text with the named Restish theme token. It returns the
// original text if the token is unknown or terminal formatting is unavailable.
func StyleText(tokenName, text string) string {
	token, err := themeTokenType(tokenName)
	if err != nil {
		return text
	}
	formatter := formatters.Get("terminal16m")
	if formatter == nil {
		return text
	}
	var out strings.Builder
	iter := chroma.Literator(chroma.Token{Type: token, Value: text})
	if err := formatter.Format(&out, restishStyle, iter); err != nil {
		return text
	}
	return out.String()
}

func themeTokenType(name string) (chroma.TokenType, error) {
	key := normalizeThemeName(name)
	if token, ok := themeTokenAliases[key]; ok {
		return token, nil
	}
	token, err := chroma.TokenTypeString(strings.TrimSpace(name))
	if err != nil {
		return 0, fmt.Errorf("theme: unknown token %q", name)
	}
	return token, nil
}

func normalizeThemeEntries(entries ThemeEntries) ThemeEntries {
	if len(entries) == 0 {
		return nil
	}
	normalized := make(ThemeEntries, len(entries))
	for name, entry := range entries {
		normalized[normalizeThemeName(name)] = entry
	}
	return normalized
}

func normalizeThemeName(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	return strings.ReplaceAll(key, "-", "_")
}
