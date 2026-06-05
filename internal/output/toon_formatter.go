package output

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TOONFormatter renders the response body as Token-Oriented Object Notation
// (TOON), a compact, indentation-based encoding of the JSON data model designed
// to reduce token usage when structured data is fed to large language models.
//
// This is an output-only encoder pinned to TOON spec v3.3
// (https://github.com/toon-format/spec). It uses the default comma delimiter
// and two-space indentation. Map-backed object keys, including tabular field
// names derived from maps, are emitted in sorted order so normal Restish output
// is deterministic.
//
// TOON's largest savings come from uniform arrays of flat, primitive-valued
// objects, which collapse into a tabular form that declares field names once and
// then streams rows. Non-uniform or nested arrays fall back to an expanded
// dash-marked list, and deeply nested or irregular data saves little versus
// JSON.
type TOONFormatter struct{}

// toonIndentUnit is the per-level indentation. The spec mandates spaces (tabs
// are prohibited for indentation) and defaults to two.
const toonIndentUnit = "  "

type toonArrayContext struct {
	allowTabular     bool
	emptyArrayHeader bool
}

type toonField struct {
	key   string
	value any
}

type toonObject []toonField

var (
	// toonUnquotedKey matches keys/field names that may be emitted without
	// quotes per spec §9.1.
	toonUnquotedKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)
	// toonNumericLike matches strings that look like numbers and therefore must
	// be quoted to stay distinct from numeric primitives.
	toonNumericLike = regexp.MustCompile(`^-?\d+(\.\d+)?([eE][+-]?\d+)?$`)
)

// Format encodes the response body as TOON. Like the JSON and gron formatters
// it renders only resp.Body; status and headers are not part of structured body
// output.
func (f *TOONFormatter) Format(w io.Writer, resp *Response, color bool) error {
	return f.FormatValue(w, resp.Body, color)
}

// FormatValue encodes an arbitrary body/sub-value as TOON. Implementing
// ValueFormatter lets filtered (-f) and paginated item values render as TOON,
// which is where the tabular form is most valuable.
func (f *TOONFormatter) FormatValue(w io.Writer, value any, color bool) error {
	out := []byte(encodeTOONDocument(value))
	// The TOON document itself ends without a trailing newline; append one for
	// terminal/pipe consistency with the json and gron formatters.
	out = append(out, '\n')
	_, err := w.Write(out)
	return err
}

// encodeTOONDocument returns a TOON document without a trailing newline.
func encodeTOONDocument(value any) string {
	var buf bytes.Buffer
	encodeTOONRoot(&buf, value)
	return strings.TrimRight(buf.String(), "\n")
}

// encodeTOONRoot writes the top-level value, which has no enclosing key.
func encodeTOONRoot(buf *bytes.Buffer, value any) {
	switch v := value.(type) {
	case map[string]any, toonObject:
		// An empty object is an empty document, which decodes back to {}.
		encodeTOONObject(buf, 0, v)
	case []any:
		if len(v) == 0 {
			buf.WriteString("[]\n")
			return
		}
		encodeTOONArray(buf, 0, "", v, toonArrayContext{allowTabular: true})
	default:
		buf.WriteString(encodeTOONScalar(value))
		buf.WriteByte('\n')
	}
}

// encodeTOONObject writes each field of obj at the given depth.
func encodeTOONObject(buf *bytes.Buffer, depth int, obj any) {
	for _, field := range toonObjectFields(obj) {
		encodeTOONField(buf, depth, field.key, field.value)
	}
}

// encodeTOONField writes a single "key: value" field and any nested content.
func encodeTOONField(buf *bytes.Buffer, depth int, key string, val any) {
	keyTok := encodeTOONKey(key)
	switch v := val.(type) {
	case map[string]any, toonObject:
		// Both empty and non-empty objects emit "key:"; a non-empty object adds
		// indented fields below.
		writeTOONIndent(buf, depth)
		buf.WriteString(keyTok)
		buf.WriteString(":\n")
		encodeTOONObject(buf, depth+1, v)
	case []any:
		encodeTOONArray(buf, depth, keyTok, v, toonArrayContext{allowTabular: true})
	default:
		writeTOONIndent(buf, depth)
		buf.WriteString(keyTok)
		buf.WriteString(": ")
		buf.WriteString(encodeTOONScalar(val))
		buf.WriteByte('\n')
	}
}

// encodeTOONArray writes an array using the most compact applicable form:
// inline for all-primitive arrays, tabular for flat primitive-valued object
// arrays with a uniform key set, and an expanded dash list otherwise. keyTok is
// the already-encoded key, or "" for a root array.
func encodeTOONArray(buf *bytes.Buffer, depth int, keyTok string, arr []any, ctx toonArrayContext) {
	writeTOONIndent(buf, depth)
	if len(arr) == 0 {
		if keyTok != "" {
			buf.WriteString(keyTok)
			buf.WriteString(": ")
			buf.WriteString("[]\n")
			return
		}
		if ctx.emptyArrayHeader {
			buf.WriteString("[0]:\n")
		} else {
			buf.WriteString("[]\n")
		}
		return
	}

	if allTOONPrimitive(arr) {
		fmt.Fprintf(buf, "%s[%d]: ", keyTok, len(arr))
		for i, item := range arr {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(encodeTOONScalar(item))
		}
		buf.WriteByte('\n')
		return
	}

	if ctx.allowTabular {
		if fields, ok := toonTabularFields(arr); ok {
			cols := make([]string, len(fields))
			for i, f := range fields {
				cols[i] = encodeTOONKey(f)
			}
			fmt.Fprintf(buf, "%s[%d]{%s}:\n", keyTok, len(arr), strings.Join(cols, ","))
			for _, item := range arr {
				objFields, _ := toonObjectFieldsIfObject(item)
				obj := toonObject(objFields)
				writeTOONIndent(buf, depth+1)
				for i, f := range fields {
					if i > 0 {
						buf.WriteByte(',')
					}
					buf.WriteString(encodeTOONScalar(toonObjectValue(obj, f)))
				}
				buf.WriteByte('\n')
			}
			return
		}
	}

	// Expanded list: header followed by one dash-marked item per element.
	fmt.Fprintf(buf, "%s[%d]:\n", keyTok, len(arr))
	for _, item := range arr {
		encodeTOONListItem(buf, depth+1, item)
	}
}

// encodeTOONListItem writes one element of an expanded list at the list-item
// depth. Composite items reuse the field/array encoders one level deeper and
// then have their first line's leading indent replaced by the "- " marker, per
// spec §9.4 and §10.
func encodeTOONListItem(buf *bytes.Buffer, depth int, item any) {
	switch v := item.(type) {
	case map[string]any, toonObject:
		fields := toonObjectFields(v)
		if len(fields) == 0 {
			writeTOONIndent(buf, depth)
			buf.WriteString("-\n")
			return
		}
		var tmp bytes.Buffer
		encodeTOONObject(&tmp, depth+1, v)
		writeTOONDashItem(buf, depth, tmp.Bytes())
	case []any:
		var tmp bytes.Buffer
		// TOON §9.4 does not allow tabular form when the nested array itself is a
		// list item; there is no key position for the field list.
		encodeTOONArray(&tmp, depth+1, "", v, toonArrayContext{
			allowTabular:     false,
			emptyArrayHeader: true,
		})
		writeTOONDashArrayItem(buf, depth, tmp.Bytes())
	default:
		writeTOONIndent(buf, depth)
		buf.WriteString("- ")
		buf.WriteString(encodeTOONScalar(item))
		buf.WriteByte('\n')
	}
}

// allTOONPrimitive reports whether every element is a primitive (not an object
// or array), making the array eligible for the compact inline form.
func allTOONPrimitive(arr []any) bool {
	for _, item := range arr {
		switch item.(type) {
		case map[string]any, toonObject, []any:
			return false
		}
	}
	return true
}

// toonTabularFields reports whether arr qualifies for tabular form and, if so,
// returns the shared field names. Qualification (spec §9.3): every element is a
// non-empty object, all share the same key set, and every value is a primitive.
func toonTabularFields(arr []any) ([]string, bool) {
	var fields []string
	for i, item := range arr {
		objFields, ok := toonObjectFieldsIfObject(item)
		if !ok || len(objFields) == 0 {
			return nil, false
		}
		for _, field := range objFields {
			switch field.value.(type) {
			case map[string]any, toonObject, []any:
				return nil, false
			}
		}
		if i == 0 {
			fields = make([]string, len(objFields))
			for j, field := range objFields {
				fields[j] = field.key
			}
			continue
		}
		if len(objFields) != len(fields) {
			return nil, false
		}
		obj := toonObject(objFields)
		for _, f := range fields {
			if _, ok := toonObjectValueOK(obj, f); !ok {
				return nil, false
			}
		}
	}
	return fields, true
}

func toonObjectFieldsIfObject(obj any) ([]toonField, bool) {
	switch v := obj.(type) {
	case map[string]any:
		return toonMapFields(v), true
	case toonObject:
		return []toonField(v), true
	default:
		return nil, false
	}
}

func toonObjectFields(obj any) []toonField {
	fields, _ := toonObjectFieldsIfObject(obj)
	return fields
}

func toonMapFields(m map[string]any) []toonField {
	keys := sortedKeys(m)
	fields := make([]toonField, len(keys))
	for i, key := range keys {
		fields[i] = toonField{key: key, value: m[key]}
	}
	return fields
}

func toonObjectValue(obj toonObject, key string) any {
	value, _ := toonObjectValueOK(obj, key)
	return value
}

func toonObjectValueOK(obj toonObject, key string) (any, bool) {
	for _, field := range obj {
		if field.key == key {
			return field.value, true
		}
	}
	return nil, false
}

// writeTOONDashItem emits content (rendered at depth+1) as a dash list item by
// swapping the first line's leading indent for the "- " marker. The marker is
// two characters wide, exactly the width of one indent level, so deeper lines
// keep their existing indentation.
func writeTOONDashItem(buf *bytes.Buffer, depth int, content []byte) {
	strip := len(toonIndentUnit) * (depth + 1)
	writeTOONIndent(buf, depth)
	buf.WriteString("- ")
	buf.Write(content[strip:])
}

// writeTOONDashArrayItem emits an array as a dash list item. The array header
// moves onto the hyphen line, so child items move up by one indentation level
// under that moved header.
func writeTOONDashArrayItem(buf *bytes.Buffer, depth int, content []byte) {
	firstLineStrip := len(toonIndentUnit) * (depth + 1)
	childStrip := []byte(toonIndentUnit)
	writeTOONIndent(buf, depth)
	buf.WriteString("- ")
	for i, line := range bytes.SplitAfter(content, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		if i == 0 {
			line = line[firstLineStrip:]
		} else if bytes.HasPrefix(line, childStrip) {
			line = line[len(childStrip):]
		}
		buf.Write(line)
	}
}

// encodeTOONScalar encodes a primitive leaf value as a TOON token, quoting and
// escaping strings as required.
func encodeTOONScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case string:
		return encodeTOONString(x)
	case float64:
		return encodeTOONFloat(x)
	case float32:
		return encodeTOONFloat(float64(x))
	case int, int8, int16, int32, int64:
		return strconv.FormatInt(reflect.ValueOf(v).Int(), 10)
	case uint, uint8, uint16, uint32, uint64:
		return strconv.FormatUint(reflect.ValueOf(v).Uint(), 10)
	case []byte:
		// Mirror JSON, which base64-encodes byte slices as strings.
		return encodeTOONString(base64.StdEncoding.EncodeToString(x))
	default:
		// json.Number satisfies fmt.Stringer; route it (and any other Stringer)
		// through the numeric normalizer when it looks numeric, else quote it.
		if s, ok := v.(fmt.Stringer); ok {
			return encodeTOONNumberString(s.String())
		}
		return encodeTOONString(fmt.Sprintf("%v", v))
	}
}

// encodeTOONFloat emits a finite float in canonical decimal form (spec §7).
// Non-finite values (NaN, ±Inf) encode as null, matching JSON-safe conversion.
func encodeTOONFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 {
		return "0" // also normalizes -0
	}
	abs := math.Abs(f)
	// Outside the canonical range exponent notation is permitted; within it the
	// spec requires plain decimal with no exponent and no trailing zeros.
	if abs < 1e-6 || abs >= 1e21 {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// encodeTOONNumberString normalizes an already-textual number (e.g. a
// json.Number). Integer literals pass through unchanged; anything with a
// fraction or exponent is re-canonicalized as a float.
func encodeTOONNumberString(s string) string {
	if s == "-0" {
		return "0"
	}
	if strings.ContainsAny(s, ".eE") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return encodeTOONFloat(f)
		}
	}
	if toonNumericLike.MatchString(s) {
		return s
	}
	return encodeTOONString(s)
}

// encodeTOONString returns s quoted and escaped if required, else verbatim.
func encodeTOONString(s string) string {
	if toonStringNeedsQuote(s) {
		return quoteTOONString(s)
	}
	return s
}

// encodeTOONKey returns a key/field name unquoted when it matches the safe
// pattern, else quoted and escaped.
func encodeTOONKey(k string) string {
	if toonUnquotedKey.MatchString(k) {
		return k
	}
	return quoteTOONString(k)
}

// toonStringNeedsQuote implements the spec's quoting triggers for the default
// comma delimiter.
func toonStringNeedsQuote(s string) bool {
	if s == "" {
		return true
	}
	if s == "true" || s == "false" || s == "null" {
		return true
	}
	if s[0] == '-' {
		return true // equals "-" or starts with a hyphen
	}
	if strings.TrimSpace(s) != s {
		return true // leading or trailing whitespace
	}
	if toonNumericLike.MatchString(s) {
		return true
	}
	if strings.ContainsAny(s, ":\"\\[]{},") {
		return true // structural characters or the active (comma) delimiter
	}
	for _, r := range s {
		if r <= 0x1f {
			return true // control characters
		}
	}
	return false
}

// quoteTOONString wraps s in double quotes and escapes per spec §7.1.
func quoteTOONString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r <= 0x1f {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// writeTOONIndent writes depth levels of indentation.
func writeTOONIndent(buf *bytes.Buffer, depth int) {
	for i := 0; i < depth; i++ {
		buf.WriteString(toonIndentUnit)
	}
}

// sortedKeys returns the keys of m in ascending order for deterministic output.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
