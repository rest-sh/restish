package plugin

import "math"

// MsgBytes coerces a CBOR-decoded value into a []byte.
// CBOR byte strings decode to []byte natively. Some CBOR implementations or
// intermediate representations may produce a string or a []any of integers
// instead; this handles all three forms so callers do not need to special-case.
func MsgBytes(v any) []byte {
	switch data := v.(type) {
	case []byte:
		return data
	case string:
		return []byte(data)
	case []any:
		out := make([]byte, 0, len(data))
		for _, item := range data {
			switch n := item.(type) {
			case uint64:
				out = append(out, byte(n))
			case int64:
				out = append(out, byte(n))
			case int:
				out = append(out, byte(n))
			case float64:
				if n >= 0 && n <= 255 && math.Trunc(n) == n {
					out = append(out, byte(n))
				}
			}
		}
		return out
	default:
		return nil
	}
}

// MsgInt coerces a CBOR-decoded value into an int.
// CBOR integers may be decoded as int, int64, uint64, or float64 depending
// on the decoder configuration and value; this handles all common forms.
func MsgInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

// MsgStrings coerces a CBOR-decoded value into a []string.
// CBOR arrays of strings decode to []any; this extracts the string items.
func MsgStrings(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}
