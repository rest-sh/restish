package output

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Custom token types for bracket-depth colorization and Restish-specific
// diagnostics.
// Brackets alternate through three colours as nesting increases.
const (
	indentLevel0 chroma.TokenType = 9000 + iota
	indentLevel1
	indentLevel2
	diagnosticInfo
	diagnosticWarn
	diagnosticError
	diagnosticHint
	httpHeaderKey
)

// indentDepthKey is the MutatorContext key used to store per-tokenise-call
// bracket nesting depth inside the chroma LexerState.
type indentDepthKey struct{}

// ReadableLexer is a custom chroma lexer for Restish readable output.
// It extends JSON tokenization with special-case patterns for:
//   - ISO 8601 / HTTP dates  → LiteralDate
//   - URLs                   → LiteralStringSymbol
//   - Hex binary ("0x…")    → LiteralNumberHex
//   - Nested bracket pairs   → alternating bracket-depth token types
//
// Ported from v1's cli/lexer.go and adapted for chroma v2 + valid JSON output
// (quoted keys, commas between items).
var ReadableLexer = lexers.Register(chroma.MustNewLexer(
	&chroma.Config{
		Name:    "Restish Readable",
		Aliases: []string{"restish-readable"},
	},
	func() chroma.Rules {
		return chroma.Rules{
			"whitespace": {
				{Pattern: `\s+`, Type: chroma.Text},
			},
			// scalar matches leaf values.
			"scalar": {
				{Pattern: `(true|false|null)\b`, Type: chroma.KeywordConstant},
				// Hex binary must come before date/number to avoid mis-matching "0x…".
				{Pattern: `"?0x[0-9a-f]+(\\.\\.\\.)?"?`, Type: chroma.LiteralNumberHex},
				// ISO 8601 date or datetime (with or without surrounding quotes).
				{Pattern: `"?[0-9]{4}-[0-9]{2}-[0-9]{2}(T[0-9:+\-.]+Z?)?"?`, Type: chroma.LiteralDate},
				// HTTP-date / RFC 1123 date (with or without surrounding quotes).
				{Pattern: `"?[A-Z][a-z]{2}, [0-9]{2} [A-Z][a-z]{2} [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} GMT"?`, Type: chroma.LiteralDate},
				{Pattern: `-?(0|[1-9]\d*)(\.\d+[eE](\+|-)?\d+|[eE](\+|-)?\d+|\.\d+)`, Type: chroma.LiteralNumberFloat},
				{Pattern: `-?(0|[1-9]\d*)`, Type: chroma.LiteralNumberInteger},
				// URL string (http/https/etc. or root-relative /).
				{Pattern: `"([a-z]+://|/)(\\\\|\\"|[^"])+"`, Type: chroma.LiteralStringSymbol},
				// All other quoted strings.
				{Pattern: `"(\\\\|\\"|[^"])*"`, Type: chroma.LiteralStringDouble},
			},
			// objectrow is active while consuming `: <value>` for a single key.
			// It pops back to "object" on newline, or skips two levels on a
			// bare closing brace (handles compact / pathological input).
			"objectrow": {
				{Pattern: `:`, Type: chroma.Punctuation},
				{Pattern: `,`, Type: chroma.Punctuation},
				{Pattern: `\n`, Type: chroma.Punctuation, Mutator: chroma.Pop(1)},
				{Pattern: `\}`, Type: chroma.Punctuation, Mutator: chroma.Pop(2)},
				chroma.Include("value"),
			},
			// object handles the keys of a JSON object.
			"object": {
				chroma.Include("whitespace"),
				// Closing brace: decrement depth and pop back to the enclosing value.
				{Pattern: `\}`, Type: chroma.EmitterFunc(readableIndentEnd), Mutator: chroma.Pop(1)},
				// Key: match everything up to (but not including) the first colon.
				// Works for both quoted ("key") and bare (key) keys.
				{Pattern: `(\\\\|\\:|[^:])+`, Type: chroma.NameTag, Mutator: chroma.Push("objectrow")},
			},
			// arrayvalue handles elements inside a JSON array.
			"arrayvalue": {
				{Pattern: `,`, Type: chroma.Punctuation},
				{Pattern: `\]`, Type: chroma.EmitterFunc(readableIndentEnd), Mutator: chroma.Pop(1)},
				chroma.Include("value"),
			},
			// value dispatches to object, array, or a scalar.
			"value": {
				chroma.Include("whitespace"),
				{Pattern: `\{`, Type: chroma.EmitterFunc(readableIndentStart), Mutator: chroma.Push("object")},
				{Pattern: `\[`, Type: chroma.EmitterFunc(readableIndentStart), Mutator: chroma.Push("arrayvalue")},
				chroma.Include("scalar"),
			},
			"root": {chroma.Include("value")},
		}
	},
))

// SchemaLexer tokenizes the compact schema language used in generated
// OpenAPI command help. It intentionally shares theme tokens with readable
// output so schema blocks embedded in Glamour-rendered help are colored like
// response bodies.
var SchemaLexer = lexers.Register(chroma.MustNewLexer(
	&chroma.Config{
		Name:    "Restish Schema",
		Aliases: []string{"schema", "restish-schema", "openapi-schema"},
	},
	func() chroma.Rules {
		return chroma.Rules{
			"whitespace": {
				{Pattern: `\s+`, Type: chroma.Text},
			},
			"scalar": {
				{Pattern: `(true|false|null)\b`, Type: chroma.KeywordConstant},
				{Pattern: `-?(0|[1-9]\d*)(\.\d+[eE](\+|-)?\d+|[eE](\+|-)?\d+|\.\d+)`, Type: chroma.LiteralNumberFloat},
				{Pattern: `-?(0|[1-9]\d*)`, Type: chroma.LiteralNumberInteger},
				{Pattern: `"([a-z]+://|/)(\\\\|\\"|[^"])+"`, Type: chroma.LiteralStringSymbol},
				{Pattern: `"(\\\\|\\"|[^"])*"`, Type: chroma.LiteralStringDouble},
				{Pattern: `<[^>\n]+>`, Type: chroma.Comment},
				{Pattern: `(allOf|oneOf|anyOf)\b`, Type: chroma.Keyword},
				{Pattern: `\(`, Type: chroma.Punctuation, Mutator: chroma.Push("annotation")},
				{Pattern: `[^{}\[\]:,\n]+`, Type: chroma.Text},
			},
			"annotation": {
				{Pattern: `\)`, Type: chroma.Punctuation, Mutator: chroma.Pop(1)},
				{Pattern: `\s+`, Type: chroma.Text},
				{Pattern: `\|`, Type: chroma.Operator},
				{Pattern: `(boolean|integer|number|string|array|object|any)\b`, Type: chroma.KeywordType},
				{Pattern: `(format|default|enum|pattern|minLen|maxLen):`, Type: chroma.NameBuiltin},
				{Pattern: `-?(0|[1-9]\d*)(\.\d+[eE](\+|-)?\d+|[eE](\+|-)?\d+|\.\d+)`, Type: chroma.LiteralNumberFloat},
				{Pattern: `-?(0|[1-9]\d*)`, Type: chroma.LiteralNumberInteger},
				{Pattern: `true|false|null\b`, Type: chroma.KeywordConstant},
				{Pattern: `[^)\s|]+`, Type: chroma.LiteralString},
			},
			"schema-key": {
				{Pattern: `\*`, Type: chroma.Operator},
				{Pattern: `[^\*:]+`, Type: chroma.NameTag},
			},
			"objectrow": {
				{Pattern: `:`, Type: chroma.Punctuation},
				{Pattern: `,`, Type: chroma.Punctuation},
				{Pattern: `\n`, Type: chroma.Punctuation, Mutator: chroma.Pop(1)},
				{Pattern: `\}`, Type: chroma.EmitterFunc(readableIndentEnd), Mutator: chroma.Pop(2)},
				chroma.Include("value"),
			},
			"object": {
				chroma.Include("whitespace"),
				{Pattern: `\}`, Type: chroma.EmitterFunc(readableIndentEnd), Mutator: chroma.Pop(1)},
				{Pattern: `([^:\n{}\[\]]+)(:)`, Type: chroma.ByGroups(chroma.UsingSelf("schema-key"), chroma.Punctuation), Mutator: chroma.Push("objectrow")},
			},
			"arrayvalue": {
				{Pattern: `,`, Type: chroma.Punctuation},
				{Pattern: `\]`, Type: chroma.EmitterFunc(readableIndentEnd), Mutator: chroma.Pop(1)},
				chroma.Include("value"),
			},
			"value": {
				chroma.Include("whitespace"),
				{Pattern: `\{`, Type: chroma.EmitterFunc(readableIndentStart), Mutator: chroma.Push("object")},
				{Pattern: `\[`, Type: chroma.EmitterFunc(readableIndentStart), Mutator: chroma.Push("arrayvalue")},
				chroma.Include("scalar"),
			},
			"root": {chroma.Include("value")},
		}
	},
))

// HTTPPreambleLexer tokenizes the status line and headers section of an HTTP
// response so they can be colored via restishStyle just like the body.
//
// Token mapping (all defined in style.go):
//
//	HTTP/x.x          → NameNamespace  (gray)
//	2xx               → GenericInserted (green)
//	3xx               → GenericOutput   (amber)
//	4xx / 5xx         → GenericError    (pink)
//	header name       → httpHeaderKey   (sky blue)
//	: separator       → Punctuation     (gray)
//	everything else   → Text
var HTTPPreambleLexer = lexers.Register(chroma.MustNewLexer(
	&chroma.Config{
		Name:    "Restish HTTP Preamble",
		Aliases: []string{"restish-http"},
	},
	func() chroma.Rules {
		return chroma.Rules{
			// statusline matches only the first line: HTTP/x.x <code> <text>\n
			"statusline": {
				{Pattern: `HTTP/\S+`, Type: chroma.NameNamespace},
				{Pattern: `[ \t]+`, Type: chroma.Text},
				{Pattern: `2\d\d`, Type: chroma.GenericInserted},
				{Pattern: `3\d\d`, Type: chroma.GenericOutput},
				{Pattern: `[45]\d\d`, Type: chroma.GenericError},
				{Pattern: `[^\n]+`, Type: chroma.Text},
				{Pattern: `\n`, Type: chroma.Text, Mutator: chroma.Push("headers")},
			},
			// headers matches header name: value lines; status code patterns are absent.
			"headers": {
				{Pattern: `([\w][\w-]*)(:)`, Type: chroma.ByGroups(httpHeaderKey, chroma.Punctuation)},
				{Pattern: `[ \t]+`, Type: chroma.Text},
				{Pattern: `[^\n]+`, Type: chroma.Text},
				{Pattern: `\n`, Type: chroma.Text},
			},
			"root": {chroma.Include("statusline")},
		}
	},
))

func readableIndentStart(groups []string, state *chroma.LexerState) chroma.Iterator {
	depth, _ := state.Get(indentDepthKey{}).(int)
	tok := chroma.Token{
		Type:  chroma.TokenType(9000 + (depth % 3)),
		Value: groups[0],
	}
	state.Set(indentDepthKey{}, depth+1)
	return chroma.Literator(tok)
}

func readableIndentEnd(groups []string, state *chroma.LexerState) chroma.Iterator {
	depth, _ := state.Get(indentDepthKey{}).(int)
	if depth > 0 {
		depth--
	}
	tok := chroma.Token{
		Type:  chroma.TokenType(9000 + (depth % 3)),
		Value: groups[0],
	}
	state.Set(indentDepthKey{}, depth)
	return chroma.Literator(tok)
}
