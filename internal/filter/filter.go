// Package filter applies shorthand or jq expressions to a response value.
package filter

import (
	"fmt"
	"strings"

	"github.com/danielgtaylor/shorthand/v2"
	"github.com/itchyny/gojq"
)

// Lang selects which query language to use.
type Lang int

const (
	// LangAuto detects the language from the expression (default).
	LangAuto Lang = iota
	// LangShorthand forces shorthand query syntax.
	LangShorthand
	// LangJQ forces jq syntax.
	LangJQ
)

// shorthandRoots are the field names recognised by the auto-detector as
// shorthand expressions. Any expression starting with one of these is sent
// to shorthand; everything else goes to jq.
var shorthandRoots = []string{"body", "headers", "links", "status", "proto", "@"}

// Apply runs expr against doc using the chosen language and returns the result.
// doc should be a map[string]any with keys "body", "headers", "links",
// "status", "proto" — i.e. the full normalised Response map.
func Apply(expr string, doc map[string]any, lang Lang) (any, error) {
	if expr == "" || expr == "@" {
		return doc, nil
	}

	switch resolve(expr, lang) {
	case LangShorthand:
		return applyShorthand(expr, doc)
	default:
		return applyJQ(expr, doc)
	}
}

// resolve returns the effective language for expr.
func resolve(expr string, lang Lang) Lang {
	if lang != LangAuto {
		return lang
	}
	for _, root := range shorthandRoots {
		if expr == root || strings.HasPrefix(expr, root+".") || strings.HasPrefix(expr, root+"[") {
			return LangShorthand
		}
	}
	return LangJQ
}

func applyShorthand(expr string, doc map[string]any) (any, error) {
	result, _, err := shorthand.GetPath(expr, doc, shorthand.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("shorthand: %w", err)
	}
	return result, nil
}

func applyJQ(expr string, doc map[string]any) (any, error) {
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("jq parse: %w", err)
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, fmt.Errorf("jq compile: %w", err)
	}

	results := make([]any, 0, 256)
	iter := code.Run(doc)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, fmt.Errorf("jq: %w", err)
		}
		results = append(results, v)
	}

	if len(results) == 0 {
		return nil, nil
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return results, nil
}
