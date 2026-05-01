package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"unicode"
)

// GronFormatter renders the body as "gron" format: each leaf value on its own
// line as `json.<path> = <value>;`. Useful for grep-friendly output.
type GronFormatter struct{}

func (f *GronFormatter) Format(w io.Writer, resp *Response, color bool) error {
	path := []byte("json")
	return gronWalk(w, &path, resp.Body)
}

// gronWalk recursively walks v and writes leaf assignments to w.
func gronWalk(w io.Writer, path *[]byte, v any) error {
	switch val := v.(type) {
	case map[string]any:
		if _, err := fmt.Fprintf(w, "%s = {};\n", *path); err != nil {
			return err
		}
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			prevLen := len(*path)
			*path = appendGronKey(*path, k)
			if err := gronWalk(w, path, val[k]); err != nil {
				return err
			}
			*path = (*path)[:prevLen]
		}
	case []any:
		if _, err := fmt.Fprintf(w, "%s = [];\n", *path); err != nil {
			return err
		}
		for i, item := range val {
			prevLen := len(*path)
			*path = append(*path, '[')
			*path = strconv.AppendInt(*path, int64(i), 10)
			*path = append(*path, ']')
			if err := gronWalk(w, path, item); err != nil {
				return err
			}
			*path = (*path)[:prevLen]
		}
	default:
		b, _ := marshalNoEscape(v)
		if _, err := fmt.Fprintf(w, "%s = %s;\n", *path, b); err != nil {
			return err
		}
	}
	return nil
}

func appendGronKey(path []byte, key string) []byte {
	if isJSIdentifier(key) {
		path = append(path, '.')
		path = append(path, key...)
		return path
	}
	encoded, _ := json.Marshal(key)
	path = append(path, '[')
	path = append(path, encoded...)
	path = append(path, ']')
	return path
}

func isJSIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && r != '$' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && r != '$' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
