// Package filter applies shorthand or jq expressions to a response value.
package filter

import (
	"container/list"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

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

func (l Lang) String() string {
	switch l {
	case LangShorthand:
		return "shorthand"
	case LangJQ:
		return "jq"
	default:
		return "auto"
	}
}

// Result is a filtered value with metadata about how the filter was evaluated.
type Result struct {
	Value any
	Lang  Lang
}

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
// doc should be a map[string]any with keys "body", "headers", "headers_all",
// "links", "status", "proto" — i.e. the full normalised Response map.
func Apply(expr string, doc map[string]any, lang Lang) (any, error) {
	result, err := ApplyWithInfo(expr, doc, lang)
	if err != nil {
		return nil, err
	}
	return result.Value, nil
}

// ApplyWithInfo runs expr against doc and returns the filtered value plus the
// language that was ultimately used.
func ApplyWithInfo(expr string, doc map[string]any, lang Lang) (Result, error) {
	if expr == "" || expr == "@" {
		return Result{Value: doc, Lang: resolvedExplicitLang(lang)}, nil
	}
	if lang != LangJQ {
		if value, ok := headerField(expr, doc); ok {
			return Result{Value: value, Lang: LangShorthand}, nil
		}
	}

	switch lang {
	case LangShorthand:
		value, err := applyShorthand(expr, doc)
		return Result{Value: value, Lang: LangShorthand}, err
	case LangJQ:
		value, err := applyJQ(expr, doc)
		return Result{Value: value, Lang: LangJQ}, err
	default:
		return applyAuto(expr, doc)
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

func resolvedExplicitLang(lang Lang) Lang {
	if lang == LangJQ {
		return LangJQ
	}
	return LangShorthand
}

func applyAuto(expr string, doc map[string]any) (Result, error) {
	shorthandResult, shorthandErr := applyShorthand(expr, doc)
	jqResult, jqErr := applyJQ(expr, doc)

	if startsWithJQField(expr) {
		if jqErr != nil {
			return Result{Lang: LangJQ}, jqErr
		}
		return Result{Value: jqResult, Lang: LangJQ}, nil
	}

	switch {
	case shorthandErr == nil && jqErr != nil:
		return Result{Value: shorthandResult, Lang: LangShorthand}, nil
	case jqErr == nil && shorthandErr != nil:
		return Result{Value: jqResult, Lang: LangJQ}, nil
	}

	preferred := preferredLang(expr)
	if shorthandErr == nil && jqErr == nil {
		if preferred == LangJQ {
			return Result{Value: jqResult, Lang: LangJQ}, nil
		}
		return Result{Value: shorthandResult, Lang: LangShorthand}, nil
	}
	if preferred == LangJQ {
		return Result{Lang: LangJQ}, errors.Join(jqErr, shorthandErr)
	}
	return Result{Lang: LangShorthand}, errors.Join(shorthandErr, jqErr)
}

func preferredLang(expr string) Lang {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return LangShorthand
	}
	shorthandScore, jqScore := 0, 0

	if startsWithBareResponseRoot(expr) {
		shorthandScore += 4
	}
	switch expr[0] {
	case '.':
		if recursiveDescentShorthand(expr) {
			shorthandScore += 5
		} else {
			jqScore += 4
		}
	case '{', '[':
		if containsJQCurrentRoot(expr) {
			jqScore += 4
		}
		if containsBareResponseRoot(expr) {
			shorthandScore += 3
		}
	}
	if hasTopLevelJQPipe(expr) {
		jqScore += 3
	}
	jqStarts := []string{
		"length", "keys", "has(", "map(", "select(", "if ", "try ", "reduce ", "foreach ",
		"true", "false", "null",
	}
	for _, start := range jqStarts {
		if expr == start || strings.HasPrefix(expr, start) {
			jqScore += 4
			break
		}
	}
	if jqScore > shorthandScore {
		return LangJQ
	}
	return LangShorthand
}

func startsWithJQField(expr string) bool {
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(expr, ".") || strings.HasPrefix(expr, "..") {
		return false
	}
	r, _ := utf8DecodeRuneInString(expr[1:])
	return isIdentStart(r)
}

func recursiveDescentShorthand(expr string) bool {
	if !strings.HasPrefix(expr, "..") || len(expr) == 2 {
		return false
	}
	r, _ := utf8DecodeRuneInString(expr[2:])
	return isIdentStart(r)
}

func startsWithBareResponseRoot(expr string) bool {
	for _, root := range responseRoots {
		if hasRootAt(expr, 0, root) {
			return true
		}
	}
	return false
}

var responseRoots = []string{"body", "headers", "links", "status", "proto"}

func containsBareResponseRoot(expr string) bool {
	for i := 0; i < len(expr); {
		r, size := utf8DecodeRuneInString(expr[i:])
		if inString, next := skipQuoted(expr, i, r); inString {
			i = next
			continue
		}
		if !isIdentStart(r) {
			i += size
			continue
		}
		for _, root := range responseRoots {
			if hasRootAt(expr, i, root) {
				return true
			}
		}
		i += size
	}
	return false
}

func hasRootAt(expr string, i int, root string) bool {
	if !strings.HasPrefix(expr[i:], root) {
		return false
	}
	beforeOK := i == 0 || !isIdentPart(rune(expr[i-1])) && expr[i-1] != '.'
	if !beforeOK {
		return false
	}
	after := i + len(root)
	if after == len(expr) {
		return true
	}
	next, _ := utf8DecodeRuneInString(expr[after:])
	if next == ':' {
		return false
	}
	return !isIdentPart(next)
}

func containsJQCurrentRoot(expr string) bool {
	for i := 0; i < len(expr); {
		r, size := utf8DecodeRuneInString(expr[i:])
		if inString, next := skipQuoted(expr, i, r); inString {
			i = next
			continue
		}
		if r != '.' || strings.HasPrefix(expr[i:], "..") {
			i += size
			continue
		}
		if i == 0 {
			return true
		}
		prev := previousNonSpace(expr, i)
		if prev < 0 || strings.ContainsRune(":{[|,(=", rune(expr[prev])) {
			return true
		}
		i += size
	}
	return false
}

func hasTopLevelJQPipe(expr string) bool {
	depth := 0
	for i := 0; i < len(expr); {
		r, size := utf8DecodeRuneInString(expr[i:])
		if inString, next := skipQuoted(expr, i, r); inString {
			i = next
			continue
		}
		switch r {
		case '[', '{', '(':
			depth++
		case ']', '}', ')':
			if depth > 0 {
				depth--
			}
		case '|':
			if depth == 0 && pipeLooksLikeJQ(expr, i) {
				return true
			}
		}
		i += size
	}
	return false
}

func pipeLooksLikeJQ(expr string, i int) bool {
	prev := previousNonSpace(expr, i)
	next := nextNonSpace(expr, i+1)
	if prev < 0 || next < 0 {
		return false
	}
	if expr[next] == '[' {
		return false
	}
	return unicode.IsSpace(rune(expr[i-1])) || unicode.IsSpace(rune(expr[i+1])) || expr[next] == '.'
}

func skipQuoted(expr string, i int, r rune) (bool, int) {
	if r != '"' && r != '\'' {
		return false, i
	}
	quote := r
	i++
	for i < len(expr) {
		next, size := utf8DecodeRuneInString(expr[i:])
		i += size
		if next == '\\' {
			if i < len(expr) {
				_, size = utf8DecodeRuneInString(expr[i:])
				i += size
			}
			continue
		}
		if next == quote {
			break
		}
	}
	return true, i
}

func previousNonSpace(expr string, i int) int {
	for j := i - 1; j >= 0; j-- {
		if !unicode.IsSpace(rune(expr[j])) {
			return j
		}
	}
	return -1
}

func nextNonSpace(expr string, i int) int {
	for j := i; j < len(expr); j++ {
		if !unicode.IsSpace(rune(expr[j])) {
			return j
		}
	}
	return -1
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r) || r == '-'
}

func utf8DecodeRuneInString(s string) (rune, int) {
	return utf8.DecodeRuneInString(s)
}

func applyShorthand(expr string, doc map[string]any) (any, error) {
	result, _, err := shorthand.GetPath(expr, doc, shorthand.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("shorthand: %w", err)
	}
	return result, nil
}

func applyJQ(expr string, doc map[string]any) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("jq: panic: %v", r)
		}
	}()
	code, err := compiledJQ(expr)
	if err != nil {
		return nil, err
	}

	results := make([]any, 0, 1)
	iter := code.Run(normalizeJQValue(doc))
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

func normalizeJQValue(value any) any {
	if value == nil {
		return nil
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return nil
		}
		return normalizeJQValue(rv.Elem().Interface())
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return value
		}
		result := make(map[string]any, rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			result[iter.Key().String()] = normalizeJQValue(iter.Value().Interface())
		}
		return result
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			return base64.StdEncoding.EncodeToString(rv.Bytes())
		}
		result := make([]any, rv.Len())
		for i := range result {
			result[i] = normalizeJQValue(rv.Index(i).Interface())
		}
		return result
	default:
		return value
	}
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
