// Package input builds HTTP request bodies from CLI arguments and/or stdin.
package input

import (
	"io"
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
func Body(stdinReader io.Reader, stdinIsTTY bool, args []string) (any, error) {
	opts := shorthand.ParseOptions{
		EnableFileInput:       true,
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
	return result, nil
}
