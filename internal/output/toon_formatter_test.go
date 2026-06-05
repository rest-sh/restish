package output_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/rest-sh/restish/v2/internal/output"
)

// renderTOON formats value through the public formatter and returns the string.
func renderTOON(t *testing.T, value any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := (&output.TOONFormatter{}).FormatValue(&buf, value, false); err != nil {
		t.Fatalf("FormatValue: %v", err)
	}
	return buf.String()
}

// TestTOONTabularArray verifies the headline case: a uniform array of objects
// collapses into a table that declares fields once (sorted) and streams rows.
func TestTOONTabularArray(t *testing.T) {
	body := []any{
		map[string]any{"id": float64(1), "name": "Alice", "role": "admin"},
		map[string]any{"id": float64(2), "name": "Bob", "role": "user"},
	}
	want := "[2]{id,name,role}:\n  1,Alice,admin\n  2,Bob,user\n"
	if got := renderTOON(t, body); got != want {
		t.Errorf("tabular output mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestTOONTabularArrayAsField verifies the tabular form under a key, which is
// the common shape after `-f` projects a response to a list of records.
func TestTOONTabularArrayAsField(t *testing.T) {
	body := map[string]any{
		"users": []any{
			map[string]any{"id": float64(1), "name": "Ada"},
			map[string]any{"id": float64(2), "name": "Bo"},
		},
	}
	want := "users[2]{id,name}:\n  1,Ada\n  2,Bo\n"
	if got := renderTOON(t, body); got != want {
		t.Errorf("tabular field mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestTOONInlineAndNested verifies inline primitive arrays and nested objects,
// with keys emitted in sorted order.
func TestTOONInlineAndNested(t *testing.T) {
	body := map[string]any{
		"tags": []any{"go", "cli"},
		"user": map[string]any{"name": "Alice", "age": float64(30)},
	}
	want := "tags[2]: go,cli\nuser:\n  age: 30\n  name: Alice\n"
	if got := renderTOON(t, body); got != want {
		t.Errorf("inline/nested mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestTOONExpandedListObjects verifies the dash-list fallback for a non-uniform
// object array, including the first field rendered on the hyphen line.
func TestTOONExpandedListObjects(t *testing.T) {
	body := []any{
		map[string]any{"a": float64(1)},
		map[string]any{"a": float64(1), "b": float64(2)},
	}
	want := "[2]:\n  - a: 1\n  - a: 1\n    b: 2\n"
	if got := renderTOON(t, body); got != want {
		t.Errorf("expanded object list mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestTOONExpandedListMixed verifies dash items for a nested array and a bare
// primitive in the same (non-uniform) array.
func TestTOONExpandedListMixed(t *testing.T) {
	body := []any{
		[]any{"x", "y"},
		"z",
	}
	want := "[2]:\n  - [2]: x,y\n  - z\n"
	if got := renderTOON(t, body); got != want {
		t.Errorf("expanded mixed list mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestTOONExpandedListArrayItemDisablesTabularForm verifies TOON §9.4: when an
// array itself is an expanded-list item, tabular form is unavailable because
// there is no key position for the field list.
func TestTOONExpandedListArrayItemDisablesTabularForm(t *testing.T) {
	body := []any{
		[]any{
			map[string]any{"a": float64(1)},
			map[string]any{"a": float64(2)},
		},
	}
	want := "[1]:\n  - [2]:\n    - a: 1\n    - a: 2\n"
	if got := renderTOON(t, body); got != want {
		t.Errorf("nested array-list item mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestTOONExpandedListEmptyObject verifies that an empty object inside a
// non-uniform array renders as a bare dash list item.
func TestTOONExpandedListEmptyObject(t *testing.T) {
	body := []any{
		map[string]any{},
		map[string]any{"a": float64(1)},
	}
	want := "[2]:\n  -\n  - a: 1\n"
	if got := renderTOON(t, body); got != want {
		t.Errorf("empty-object list item mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// TestTOONEmptyContainers verifies empty arrays/objects at root and field
// positions.
func TestTOONEmptyContainers(t *testing.T) {
	cases := []struct {
		name string
		body any
		want string
	}{
		{"empty object root", map[string]any{}, "\n"},
		{"empty array root", []any{}, "[]\n"},
		{"empty array field", map[string]any{"items": []any{}}, "items: []\n"},
		{"empty object field", map[string]any{"meta": map[string]any{}}, "meta:\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderTOON(t, tc.body); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTOONScalars verifies primitive encoding: null/bool literals, number
// canonicalization, and the string-quoting triggers.
func TestTOONScalars(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  string
	}{
		{"null", nil, "null\n"},
		{"bool", true, "true\n"},
		{"plain string", "hello", "hello\n"},
		{"float", 1.5, "1.5\n"},
		{"whole float is integer", 1.0, "1\n"},
		{"no exponent in range", 1000000.0, "1000000\n"},
		{"negative zero normalized", math.Copysign(0, -1), "0\n"},
		{"int64", int64(42), "42\n"},
		{"uint64", uint64(7), "7\n"},
		{"float32", float32(1.5), "1.5\n"},
		{"byte slice as base64", []byte("hi"), "aGk=\n"},
		{"NaN encodes as null", math.NaN(), "null\n"},
		{"positive infinity as null", math.Inf(1), "null\n"},
		{"negative infinity as null", math.Inf(-1), "null\n"},
		{"large magnitude uses exponent", 1e21, "1e+21\n"},
		{"tiny magnitude uses exponent", 1e-7, "1e-07\n"},
		{"control char escaped", "a\x01b", "\"a\\u0001b\"\n"},
		{"json.Number int", json.Number("42"), "42\n"},
		{"json.Number big int beyond float64", json.Number("123456789012345678901"), "123456789012345678901\n"},
		{"json.Number trailing zeros", json.Number("1.500"), "1.5\n"},
		{"quote on delimiter", "a,b", "\"a,b\"\n"},
		{"quote keyword lookalike", "true", "\"true\"\n"},
		{"quote leading hyphen", "-x", "\"-x\"\n"},
		{"quote numeric lookalike", "123", "\"123\"\n"},
		{"quote colon", "a:b", "\"a:b\"\n"},
		{"escape quote and newline", "a\"\nb", "\"a\\\"\\nb\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderTOON(t, tc.value); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTOONKeyQuoting verifies that field names are quoted only when they fall
// outside the safe-identifier pattern (dots are allowed; spaces and a leading
// digit are not).
func TestTOONKeyQuoting(t *testing.T) {
	cases := []struct {
		body map[string]any
		want string
	}{
		{map[string]any{"a.b": float64(1)}, "a.b: 1\n"},
		{map[string]any{"a b": float64(1)}, "\"a b\": 1\n"},
		{map[string]any{"1foo": float64(1)}, "\"1foo\": 1\n"},
	}
	for _, tc := range cases {
		if got := renderTOON(t, tc.body); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

// TestTOONFormatRendersBodyOnly verifies Format encodes only resp.Body, like the
// json and gron formatters, without any status/header preamble.
func TestTOONFormatRendersBodyOnly(t *testing.T) {
	resp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  200,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    map[string]any{"ok": true},
	}
	var buf bytes.Buffer
	if err := (&output.TOONFormatter{}).Format(&buf, resp, false); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if got, want := buf.String(), "ok: true\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTOONFormatterPropagatesWriterError(t *testing.T) {
	err := (&output.TOONFormatter{}).FormatValue(failingWriter{}, map[string]any{"x": float64(1)}, false)
	if !errors.Is(err, errFailingWriter) {
		t.Fatalf("error = %v, want %v", err, errFailingWriter)
	}
}
