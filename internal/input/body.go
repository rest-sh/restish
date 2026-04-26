// Package input builds HTTP request bodies from CLI arguments and/or stdin.
package input

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/danielgtaylor/shorthand/v2"
)

// Body parses positional CLI args (shorthand syntax) and/or stdin into a
// request body. The rules are:
//
//  1. No args, stdin is a TTY → nil body (no Content-Type sent).
//  2. No args, data on stdin → parsed as shorthand/JSON/YAML and returned
//     as a Go value so the content registry can marshal it correctly.
//  3. Args only → joined with spaces (matches how the shell splits tokens)
//     and parsed as shorthand.
//  4. Stdin + args → stdin document is the base; args are applied as a
//     shorthand patch on top.
//
// The returned body is a Go value (map/slice/scalar) ready to be marshalled
// by the content registry, or nil if there is no body.
// stdinReader is cli.Stdin; pass strings.NewReader("") with stdinIsTTY=true
// in tests to simulate an empty terminal.
func Body(stdinReader io.Reader, stdinIsTTY bool, args []string, contentType string) (any, error) {
	return BodyWithSchemaTypes(stdinReader, stdinIsTTY, args, contentType, nil)
}

// BodyWithSchemaTypes parses a request body like Body, then coerces values for
// dotted paths whose schema type is known. This is used by generated commands
// so OpenAPI string fields keep string semantics for numeric-looking shorthand
// values without changing generic request shorthand behavior.
func BodyWithSchemaTypes(stdinReader io.Reader, stdinIsTTY bool, args []string, contentType string, schemaTypes map[string]string) (any, error) {
	opts := shorthand.ParseOptions{
		EnableFileInput:       enableFileInput(contentType),
		EnableObjectDetection: true,
	}

	var base any

	if !stdinIsTTY {
		data, err := io.ReadAll(stdinReader)
		if err != nil {
			return nil, err
		}
		if len(data) > 0 {
			parsed, serr := shorthand.Unmarshal(string(data), opts, nil)
			if serr != nil {
				// Not parseable as structured data — return raw string so the
				// content registry passes it through as the request body.
				if len(args) == 0 {
					return string(data), nil
				}
				// Can't patch non-structured stdin; treat it as args-only.
			} else {
				if len(args) == 0 {
					coerceSchemaTypes(parsed, schemaTypes)
					return parsed, nil
				}
				base = parsed
			}
		}
	}

	if len(args) == 0 {
		return nil, nil
	}

	// Args are shell-split tokens; join with spaces to reconstruct the
	// full shorthand expression (e.g. ["name:", "Alice,", "age:", "30"]
	// → "name: Alice, age: 30").
	result, err := shorthand.Unmarshal(strings.Join(args, " "), opts, base)
	if err != nil {
		return nil, err
	}
	coerceSchemaTypes(result, schemaTypes)
	return result, nil
}

func coerceSchemaTypes(value any, schemaTypes map[string]string) {
	if len(schemaTypes) == 0 {
		return
	}
	for path, typ := range schemaTypes {
		if typ != "string" {
			continue
		}
		coercePathToString(value, strings.Split(path, "."))
	}
}

func coercePathToString(value any, parts []string) {
	if len(parts) == 0 {
		return
	}
	switch m := value.(type) {
	case map[string]any:
		coerceStringMapPath(m, parts)
	case map[any]any:
		coerceAnyMapPath(m, parts)
	}
}

func coerceStringMapPath(m map[string]any, parts []string) {
	if len(parts) == 1 {
		if v, ok := m[parts[0]]; ok {
			m[parts[0]] = schemaString(v)
		}
		return
	}
	next, ok := m[parts[0]]
	if !ok {
		return
	}
	coercePathToString(next, parts[1:])
}

func coerceAnyMapPath(m map[any]any, parts []string) {
	key := any(parts[0])
	value, ok := m[key]
	if !ok {
		return
	}
	if len(parts) == 1 {
		m[key] = schemaString(value)
		return
	}
	coercePathToString(value, parts[1:])
}

func schemaString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func enableFileInput(contentType string) bool {
	switch strings.ToLower(contentType) {
	case "form", "multipart", "application/x-www-form-urlencoded", "multipart/form-data":
		return false
	default:
		return true
	}
}
