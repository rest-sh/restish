// Package filter applies shorthand or jq expressions to a response value.
package filter

import (
	"container/list"
	"errors"
	"fmt"
	"strings"
	"sync"

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

// jqCacheMaxSize caps the number of compiled jq programs kept in the global
// cache so long-running embedders don't grow unboundedly.
const jqCacheMaxSize = 1024

var (
	jqCacheMu sync.Mutex
	jqCache   = make(map[string]*list.Element, jqCacheMaxSize)
	jqLRU     = list.New()
)

type jqCacheEntry struct {
	expr string
	code *gojq.Code
}

// Apply runs expr against doc using the chosen language and returns the result.
// doc should be a map[string]any with keys "body", "headers", "links",
// "status", "proto" — i.e. the full normalised Response map.
func Apply(expr string, doc map[string]any, lang Lang) (any, error) {
	if expr == "" || expr == "@" {
		return doc, nil
	}
	if lang != LangJQ {
		if value, ok := headerField(expr, doc); ok {
			return value, nil
		}
	}

	switch resolve(expr, lang) {
	case LangShorthand:
		return applyShorthand(expr, doc)
	default:
		result, err := applyJQ(expr, doc)
		if err == nil || lang != LangAuto || !strings.Contains(err.Error(), "jq parse:") {
			return result, err
		}
		shorthandResult, shorthandErr := applyShorthand(expr, doc)
		if shorthandErr != nil {
			return nil, errors.Join(err, shorthandErr)
		}
		return shorthandResult, nil
	}
}

func headerField(expr string, doc map[string]any) (any, bool) {
	name, ok := strings.CutPrefix(expr, "headers.")
	if !ok || name == "" || strings.ContainsAny(name, "[]|") {
		return nil, false
	}
	switch headers := doc["headers"].(type) {
	case map[string]string:
		for key, value := range headers {
			if strings.EqualFold(key, name) {
				return value, true
			}
		}
	case map[string]any:
		for key, value := range headers {
			if strings.EqualFold(key, name) {
				return value, true
			}
		}
	case map[string][]string:
		for key, values := range headers {
			if strings.EqualFold(key, name) {
				return strings.Join(values, ","), true
			}
		}
	}
	return nil, false
}

// resolve returns the effective language for expr.
func resolve(expr string, lang Lang) Lang {
	if lang != LangAuto {
		return lang
	}
	trimmed := strings.TrimSpace(expr)
	if looksLikeJQ(trimmed) {
		return LangJQ
	}
	return LangShorthand
}

func looksLikeJQ(expr string) bool {
	if expr == "" {
		return false
	}
	switch expr[0] {
	case '.', '{', '[':
		return true
	}
	jqStarts := []string{
		"length", "keys", "has(", "map(", "select(", "if ", "try ", "reduce ", "foreach ",
		"true", "false", "null",
	}
	for _, start := range jqStarts {
		if expr == start || strings.HasPrefix(expr, start) {
			return true
		}
	}
	return false
}

func applyShorthand(expr string, doc map[string]any) (any, error) {
	result, _, err := shorthand.GetPath(expr, doc, shorthand.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("shorthand: %w", err)
	}
	return result, nil
}

func applyJQ(expr string, doc map[string]any) (any, error) {
	code, err := compiledJQ(expr)
	if err != nil {
		return nil, err
	}

	results := make([]any, 0, 1)
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

func compiledJQ(expr string) (*gojq.Code, error) {
	jqCacheMu.Lock()
	if elem, ok := jqCache[expr]; ok {
		jqLRU.MoveToBack(elem)
		code := elem.Value.(jqCacheEntry).code
		jqCacheMu.Unlock()
		return code, nil
	}
	jqCacheMu.Unlock()

	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("jq parse: %w", err)
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return nil, fmt.Errorf("jq compile: %w", err)
	}

	jqCacheMu.Lock()
	if elem, ok := jqCache[expr]; ok {
		jqLRU.MoveToBack(elem)
		existing := elem.Value.(jqCacheEntry).code
		jqCacheMu.Unlock()
		return existing, nil
	}
	for len(jqCache) >= jqCacheMaxSize {
		oldest := jqLRU.Front()
		if oldest == nil {
			break
		}
		entry := oldest.Value.(jqCacheEntry)
		delete(jqCache, entry.expr)
		jqLRU.Remove(oldest)
	}
	jqCache[expr] = jqLRU.PushBack(jqCacheEntry{expr: expr, code: code})
	jqCacheMu.Unlock()
	return code, nil
}
