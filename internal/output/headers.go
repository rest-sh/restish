package output

import "strings"

// Header returns the first value for name from headers, matching
// http.Header.Get's case-insensitive lookup semantics.
func Header(headers map[string][]string, name string) string {
	for k, values := range headers {
		if strings.EqualFold(k, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// HeaderValues returns all values for name from headers.
func HeaderValues(headers map[string][]string, name string) []string {
	for k, values := range headers {
		if strings.EqualFold(k, name) {
			return append([]string(nil), values...)
		}
	}
	return nil
}
