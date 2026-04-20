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
	gronWalk(w, &path, resp.Body)
	return nil
}

// gronWalk recursively walks v and writes leaf assignments to w.
func gronWalk(w io.Writer, path *[]byte, v any) {
	switch val := v.(type) {
	case map[string]any:
		fmt.Fprintf(w, "%s = {};\n", *path)
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			prevLen := len(*path)
			*path = appendGronKey(*path, k)
			gronWalk(w, path, val[k])
			*path = (*path)[:prevLen]
		}
	case []any:
		fmt.Fprintf(w, "%s = [];\n", *path)
		for i, item := range val {
			prevLen := len(*path)
			*path = append(*path, '[')
			*path = strconv.AppendInt(*path, int64(i), 10)
			*path = append(*path, ']')
			gronWalk(w, path, item)
			*path = (*path)[:prevLen]
		}
	default:
		b, _ := marshalNoEscape(v)
		fmt.Fprintf(w, "%s = %s;\n", *path, b)
	}
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
