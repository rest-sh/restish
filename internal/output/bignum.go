package output

import (
	"encoding/json"
	"fmt"
	"math/big"

	"go.yaml.in/yaml/v3"
)

// Response bodies keep integers that do not fit int64/uint64 as json.Number
// (JSON decoding) or *big.Int (jq results). Neither YAML nor CBOR knows those
// types, so convert them into each encoder's native big-number form instead of
// letting them fall back to strings or structs.

func hasBigNumbers(v any) bool {
	switch value := v.(type) {
	case json.Number, *big.Int:
		return true
	case []any:
		for _, item := range value {
			if hasBigNumbers(item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range value {
			if hasBigNumbers(item) {
				return true
			}
		}
	}
	return false
}

// mapBigNumbers returns a copy of v with every json.Number and *big.Int
// replaced by convert's result. The input is not modified.
func mapBigNumbers(v any, convert func(any) any) any {
	switch value := v.(type) {
	case json.Number, *big.Int:
		return convert(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = mapBigNumbers(item, convert)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(value))
		for key, item := range value {
			out[key] = mapBigNumbers(item, convert)
		}
		return out
	}
	return v
}

// yamlBigNumbers renders big numbers as plain (unquoted) YAML scalars.
func yamlBigNumbers(v any) any {
	if !hasBigNumbers(v) {
		return v
	}
	return mapBigNumbers(v, func(n any) any {
		// An empty tag lets the encoder resolve the scalar itself: numeric
		// text is emitted plain, with no quotes and no explicit !!int tag
		// (which YAML 1.2 parsers would otherwise need for out-of-range ints).
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(n)}
	})
}

// cborBigNumbers converts integer json.Number values to *big.Int so they are
// encoded as CBOR bignums (tags 2/3) rather than text strings.
func cborBigNumbers(v any) any {
	if !hasBigNumbers(v) {
		return v
	}
	return mapBigNumbers(v, func(n any) any {
		if num, ok := n.(json.Number); ok {
			if i, ok := new(big.Int).SetString(num.String(), 10); ok {
				return i
			}
		}
		return n
	})
}
