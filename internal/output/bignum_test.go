package output

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/fxamacker/cbor/v2"
)

func TestBigIntegersKeepNumericTypeInYAMLAndCBOR(t *testing.T) {
	jqBig, _ := new(big.Int).SetString("-123456789012345678901234567890", 10)
	body := map[string]any{
		"big":   json.Number("123456789012345678901234567890"),
		"below": json.Number("-9223372036854775809"),
		"jq":    jqBig,
		"list":  []any{json.Number("98765432109876543210"), 1.5},
	}

	var yamlOut bytes.Buffer
	if err := (&YAMLFormatter{}).Format(&yamlOut, &Response{Body: body}, false); err != nil {
		t.Fatalf("yaml: %v", err)
	}
	want := "below: -9223372036854775809\n" +
		"big: 123456789012345678901234567890\n" +
		"jq: -123456789012345678901234567890\n" +
		"list:\n    - 98765432109876543210\n    - 1.5\n"
	if got := yamlOut.String(); got != want {
		t.Fatalf("yaml =\n%s\nwant\n%s", got, want)
	}

	var cborOut bytes.Buffer
	if err := (&CBORFormatter{}).Format(&cborOut, &Response{Body: body}, false); err != nil {
		t.Fatalf("cbor: %v", err)
	}
	var decoded map[string]any
	if err := cbor.Unmarshal(cborOut.Bytes(), &decoded); err != nil {
		t.Fatalf("decode cbor: %v", err)
	}
	for key, want := range map[string]string{
		"big":   "123456789012345678901234567890",
		"below": "-9223372036854775809",
		"jq":    "-123456789012345678901234567890",
	} {
		got, ok := decoded[key].(big.Int)
		if !ok {
			t.Fatalf("cbor %s = %#v, want big.Int bignum", key, decoded[key])
		}
		if got.String() != want {
			t.Fatalf("cbor %s = %s, want %s", key, got.String(), want)
		}
	}
	if _, ok := body["big"].(json.Number); !ok {
		t.Fatalf("formatters must not modify the response body")
	}
}
