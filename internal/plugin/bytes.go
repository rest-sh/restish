package plugin

// MsgBytes coerces a CBOR-decoded value into a []byte.
// CBOR byte strings decode to []byte natively. Some CBOR implementations or
// intermediate representations may produce a string or a []any of integers
// instead; this handles all three forms so callers don't need to special-case.
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
			}
		}
		return out
	default:
		return nil
	}
}
