package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// GronFormatter renders the body as "gron" format: each leaf value on its own
// line as `json.<path> = <value>;`. Useful for grep-friendly output.
type GronFormatter struct{}

func (f *GronFormatter) Format(w io.Writer, resp *Response, color bool) error {
	gronWalk(w, "json", resp.Body)
	return nil
}

// gronWalk recursively walks v and writes leaf assignments to w.
func gronWalk(w io.Writer, path string, v any) {
	switch val := v.(type) {
	case map[string]any:
		fmt.Fprintf(w, "%s = {};\n", path)
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			gronWalk(w, path+"."+k, val[k])
		}
	case []any:
		fmt.Fprintf(w, "%s = [];\n", path)
		for i, item := range val {
			gronWalk(w, path+"["+strconv.Itoa(i)+"]", item)
		}
	default:
		b, _ := json.Marshal(v)
		fmt.Fprintf(w, "%s = %s;\n", path, b)
	}
}
