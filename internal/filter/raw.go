package filter

import (
	"fmt"
	"strings"
)

// RawOutput formats v for --rsh-raw output:
//   - string → value without surrounding quotes
//   - []any of scalars → one value per line, strings unquoted
//   - anything else → fmt.Sprintf %v
func RawOutput(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []any:
		lines := make([]string, len(val))
		for i, item := range val {
			if s, ok := item.(string); ok {
				lines[i] = s
			} else {
				lines[i] = fmt.Sprintf("%v", item)
			}
		}
		return strings.Join(lines, "\n")
	default:
		return fmt.Sprintf("%v", val)
	}
}
