package output

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
)

// restishStyle is a 256-color terminal theme for Restish readable output.
// It assigns colors to all token types produced by ReadableLexer as well as
// standard JSON token types.
var restishStyle = styles.Register(chroma.MustNewStyle("restish", chroma.StyleEntries{
	// Core JSON token types
	chroma.Comment:             "#9e9e9e",
	chroma.KeywordConstant:     "#ff5f87", // true / false / null
	chroma.Punctuation:         "#9e9e9e", // : , braces handled by indent tokens
	chroma.NameTag:             "#5fafd7", // object keys
	chroma.LiteralNumber:       "#d78700",
	chroma.LiteralNumberFloat:  "#d78700",
	chroma.LiteralNumberInteger: "#d78700",
	chroma.LiteralString:       "#afd787",
	chroma.LiteralStringDouble: "#afd787",

	// Special value types detected by ReadableLexer
	chroma.LiteralStringSymbol: "italic #D6FFB7", // URLs
	chroma.LiteralDate:         "#af87af",         // ISO 8601 dates
	chroma.LiteralNumberHex:    "#ffd7d7",         // hex binary (0x...)

	// Bracket-depth colorization: IndentLevel0/1/2 cycle through three colours.
	IndentLevel0: "#d78700", // amber
	IndentLevel1: "#af87af", // mauve
	IndentLevel2: "#5fafd7", // sky blue

	// HTTP preamble (status line + headers via HTTPPreambleLexer)
	chroma.NameNamespace:    "#9e9e9e",         // HTTP/x.x  → gray
	chroma.GenericInserted:  "bold #afd787",    // 2xx       → bold light-green
	chroma.GenericOutput:    "bold #d78700",    // 3xx       → bold amber
	chroma.GenericError:     "bold #ff5f87",    // 4xx/5xx   → bold pink
}))
