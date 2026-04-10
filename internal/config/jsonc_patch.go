package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type jsoncValue struct {
	kind valueKind

	start int
	end   int
}

type valueKind int

const (
	valueObject valueKind = iota
	valueArray
	valueString
	valueNumber
	valueLiteral
)

type jsoncObject struct {
	lbrace  int
	rbrace  int
	members []jsoncMember
}

type jsoncMember struct {
	key string

	start int
	end   int
	comma int

	keyStart int
	value    jsoncValue
}

// SaveAPIConfig updates a single API entry in the JSONC config file while
// preserving surrounding comments and formatting when possible.
func SaveAPIConfig(path, apiName string, apiCfg *APIConfig) error {
	return patchConfig(path, func(data []byte) ([]byte, error) {
		return jsoncSetPath(data, []string{"apis", apiName}, apiCfg)
	})
}

// DeleteAPIConfig removes a single API entry from the JSONC config file while
// preserving surrounding comments and formatting when possible.
func DeleteAPIConfig(path, apiName string) error {
	return patchConfig(path, func(data []byte) ([]byte, error) {
		return jsoncDeletePath(data, []string{"apis", apiName})
	})
}

// SaveConfigValue updates a single object path inside the JSONC config file
// while preserving surrounding comments and formatting when possible.
func SaveConfigValue(path string, objectPath []string, value any) error {
	return patchConfig(path, func(data []byte) ([]byte, error) {
		return jsoncSetPath(data, objectPath, value)
	})
}

func patchConfig(path string, patch func([]byte) ([]byte, error)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("config: cannot read %s: %w", path, err)
	}

	patched, err := patch(data)
	if err != nil {
		return err
	}
	if _, err := parseConfigBytes(path, patched); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	return os.WriteFile(path, patched, 0o600)
}

func jsoncSetPath(data []byte, path []string, value any) ([]byte, error) {
	root, err := parseRootObject(data)
	if err != nil {
		return nil, err
	}
	return setObjectPath(data, root, path, value, guessIndentUnit(data))
}

func jsoncDeletePath(data []byte, path []string) ([]byte, error) {
	root, err := parseRootObject(data)
	if err != nil {
		return nil, err
	}
	return deleteObjectPath(data, root, path)
}

func setObjectPath(data []byte, obj *jsoncObject, path []string, value any, indentUnit string) ([]byte, error) {
	if len(path) == 0 {
		return data, nil
	}

	member := obj.member(path[0])
	if len(path) == 1 {
		raw, err := marshalJSONCValue(value, memberIndent(data, obj, member, indentUnit), indentUnit)
		if err != nil {
			return nil, err
		}
		if member != nil {
			return replaceSpan(data, member.value.start, member.value.end, raw), nil
		}
		return insertObjectMember(data, obj, path[0], raw, indentUnit)
	}

	if member == nil {
		raw, err := buildNestedObject(path[1:], value, memberIndent(data, obj, nil, indentUnit), indentUnit)
		if err != nil {
			return nil, err
		}
		return insertObjectMember(data, obj, path[0], raw, indentUnit)
	}

	if member.value.kind != valueObject {
		raw, err := buildNestedObject(path[1:], value, memberIndent(data, obj, member, indentUnit), indentUnit)
		if err != nil {
			return nil, err
		}
		return replaceSpan(data, member.value.start, member.value.end, raw), nil
	}

	child, err := parseObjectAt(data, member.value.start)
	if err != nil {
		return nil, err
	}
	return setObjectPath(data, child, path[1:], value, indentUnit)
}

func deleteObjectPath(data []byte, obj *jsoncObject, path []string) ([]byte, error) {
	if len(path) == 0 {
		return data, nil
	}
	member := obj.member(path[0])
	if member == nil {
		return data, nil
	}
	if len(path) > 1 {
		if member.value.kind != valueObject {
			return data, nil
		}
		child, err := parseObjectAt(data, member.value.start)
		if err != nil {
			return nil, err
		}
		return deleteObjectPath(data, child, path[1:])
	}

	start, end := memberRemovalSpan(obj, member)
	return append(append([]byte{}, data[:start]...), data[end:]...), nil
}

func (o *jsoncObject) member(key string) *jsoncMember {
	for i := range o.members {
		if o.members[i].key == key {
			return &o.members[i]
		}
	}
	return nil
}

func memberRemovalSpan(obj *jsoncObject, member *jsoncMember) (int, int) {
	index := -1
	for i := range obj.members {
		if obj.members[i].key == member.key {
			index = i
			break
		}
	}
	if index <= 0 {
		if member.comma >= 0 {
			return member.start, member.comma + 1
		}
		return member.start, member.end
	}
	prev := obj.members[index-1]
	if member.comma >= 0 {
		return prev.comma + 1, member.end
	}
	return prev.comma, member.end
}

func insertObjectMember(data []byte, obj *jsoncObject, key string, raw []byte, indentUnit string) ([]byte, error) {
	keyRaw, err := json.Marshal(key)
	if err != nil {
		return nil, fmt.Errorf("config: marshal key: %w", err)
	}

	if len(obj.members) == 0 {
		if isInlineObject(data, obj) && !bytes.Contains(raw, []byte("\n")) {
			member := append(append(append([]byte{}, keyRaw...), ':', ' '), raw...)
			return replaceSpan(data, obj.rbrace, obj.rbrace, member), nil
		}
		baseIndent := lineIndent(data, obj.rbrace)
		childIndent := baseIndent + indentUnit
		newline := []byte("\n")
		if bytes.Contains(data, []byte("\r\n")) {
			newline = []byte("\r\n")
		}
		member := append(append(append([]byte{}, newline...), []byte(childIndent)...), keyRaw...)
		member = append(member, ':', ' ')
		member = append(member, raw...)
		member = append(member, newline...)
		member = append(member, []byte(baseIndent)...)
		return replaceSpan(data, obj.lbrace+1, obj.rbrace, member), nil
	}

	if isInlineObject(data, obj) {
		member := append([]byte(", "), keyRaw...)
		member = append(member, ':', ' ')
		member = append(member, raw...)
		return replaceSpan(data, obj.rbrace, obj.rbrace, member), nil
	}

	last := obj.members[len(obj.members)-1]
	childIndent := memberIndent(data, obj, &last, indentUnit)
	member := append([]byte(",\n"), []byte(childIndent)...)
	member = append(member, keyRaw...)
	member = append(member, ':', ' ')
	member = append(member, raw...)
	insertAt := closingLineStart(data, obj.rbrace)
	return replaceSpan(data, insertAt, insertAt, member), nil
}

func buildNestedObject(path []string, value any, baseIndent, indentUnit string) ([]byte, error) {
	raw, err := marshalJSONCValue(value, baseIndent, indentUnit)
	if err != nil {
		return nil, err
	}
	for i := len(path) - 1; i >= 0; i-- {
		keyRaw, err := json.Marshal(path[i])
		if err != nil {
			return nil, fmt.Errorf("config: marshal key: %w", err)
		}
		raw = append(append(append([]byte{'{'}, keyRaw...), ':', ' '), raw...)
		raw = append(raw, '}')
	}
	return raw, nil
}

func marshalJSONCValue(value any, baseIndent, indentUnit string) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", indentUnit)
	if err != nil {
		return nil, fmt.Errorf("config: marshal: %w", err)
	}
	if !bytes.Contains(raw, []byte("\n")) {
		return raw, nil
	}
	return []byte(indentMultiline(string(raw), baseIndent)), nil
}

func indentMultiline(s, indent string) string {
	if !strings.Contains(s, "\n") {
		return s
	}
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}

func replaceSpan(data []byte, start, end int, replacement []byte) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(replacement))
	out = append(out, data[:start]...)
	out = append(out, replacement...)
	out = append(out, data[end:]...)
	return out
}

func memberIndent(data []byte, obj *jsoncObject, member *jsoncMember, indentUnit string) string {
	if member != nil {
		return lineIndent(data, member.keyStart)
	}
	return lineIndent(data, obj.rbrace) + indentUnit
}

func lineIndent(data []byte, pos int) string {
	start := pos
	for start > 0 && data[start-1] != '\n' && data[start-1] != '\r' {
		start--
	}
	end := start
	for end < pos && (data[end] == ' ' || data[end] == '\t') {
		end++
	}
	return string(data[start:end])
}

func closingLineStart(data []byte, pos int) int {
	i := pos
	for i > 0 && (data[i-1] == ' ' || data[i-1] == '\t') {
		i--
	}
	if i > 0 && (data[i-1] == '\n' || data[i-1] == '\r') {
		return i - 1
	}
	return pos
}

func isInlineObject(data []byte, obj *jsoncObject) bool {
	return !bytes.Contains(data[obj.lbrace:obj.rbrace], []byte("\n"))
}

func guessIndentUnit(data []byte) string {
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		i := 0
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i > 0 {
			return string(line[:i])
		}
	}
	return "  "
}

func parseRootObject(data []byte) (*jsoncObject, error) {
	obj, end, err := parseObjectWithEnd(data, 0)
	if err != nil {
		return nil, err
	}
	parser := jsoncParser{data: data}
	end, err = parser.skipTrivia(end)
	if err != nil {
		return nil, err
	}
	if end != len(data) {
		return nil, fmt.Errorf("config: unexpected trailing data at byte %d", end)
	}
	return obj, nil
}

func parseObjectAt(data []byte, start int) (*jsoncObject, error) {
	obj, _, err := parseObjectWithEnd(data, start)
	return obj, err
}

func parseObjectWithEnd(data []byte, start int) (*jsoncObject, int, error) {
	parser := jsoncParser{data: data}
	start, err := parser.skipTrivia(start)
	if err != nil {
		return nil, 0, err
	}
	if start >= len(data) || data[start] != '{' {
		return nil, 0, fmt.Errorf("config: expected object at byte %d", start)
	}
	return parser.parseObjectMembers(start)
}

func parseJSONC(data []byte) (*jsoncValue, error) {
	parser := jsoncParser{data: data}
	value, err := parser.skipValue(0)
	if err != nil {
		return nil, err
	}
	end, err := parser.skipTrivia(value.end)
	if err != nil {
		return nil, err
	}
	if end != len(data) {
		return nil, fmt.Errorf("config: unexpected trailing data at byte %d", end)
	}
	return &value, nil
}

type jsoncParser struct {
	data []byte
}

func (p *jsoncParser) parseObjectMembers(i int) (*jsoncObject, int, error) {
	obj := &jsoncObject{lbrace: i}
	i++
	for {
		memberStart, err := p.skipTrivia(i)
		if err != nil {
			return nil, 0, err
		}
		if memberStart >= len(p.data) {
			return nil, 0, fmt.Errorf("config: unterminated object")
		}
		if p.data[memberStart] == '}' {
			obj.rbrace = memberStart
			return obj, memberStart + 1, nil
		}

		keyEnd, err := p.parseString(memberStart)
		if err != nil {
			return nil, 0, err
		}
		var key string
		if err := json.Unmarshal(p.data[memberStart:keyEnd], &key); err != nil {
			return nil, 0, fmt.Errorf("config: invalid object key: %w", err)
		}
		colon, err := p.skipTrivia(keyEnd)
		if err != nil {
			return nil, 0, err
		}
		if colon >= len(p.data) || p.data[colon] != ':' {
			return nil, 0, fmt.Errorf("config: expected ':' after object key")
		}
		value, err := p.skipValue(colon + 1)
		if err != nil {
			return nil, 0, err
		}
		memberEnd, err := p.skipTrivia(value.end)
		if err != nil {
			return nil, 0, err
		}
		member := jsoncMember{
			key:      key,
			start:    memberStart,
			end:      memberEnd,
			comma:    -1,
			keyStart: memberStart,
			value:    value,
		}
		if memberEnd < len(p.data) && p.data[memberEnd] == ',' {
			member.comma = memberEnd
			member.end = memberEnd + 1
			i = memberEnd + 1
		} else {
			i = memberEnd
		}
		obj.members = append(obj.members, member)
	}
}

func (p *jsoncParser) skipValue(i int) (jsoncValue, error) {
	start, err := p.skipTrivia(i)
	if err != nil {
		return jsoncValue{}, err
	}
	if start >= len(p.data) {
		return jsoncValue{}, fmt.Errorf("config: unexpected end of JSONC input")
	}
	switch p.data[start] {
	case '{':
		return p.skipObject(start)
	case '[':
		return p.skipArray(start)
	case '"':
		end, err := p.parseString(start)
		return jsoncValue{kind: valueString, start: start, end: end}, err
	case 't', 'f', 'n':
		end, err := p.parseLiteral(start)
		return jsoncValue{kind: valueLiteral, start: start, end: end}, err
	default:
		end, err := p.parseNumber(start)
		return jsoncValue{kind: valueNumber, start: start, end: end}, err
	}
}

func (p *jsoncParser) skipObject(i int) (jsoncValue, error) {
	start := i
	i++
	for {
		memberStart, err := p.skipTrivia(i)
		if err != nil {
			return jsoncValue{}, err
		}
		if memberStart >= len(p.data) {
			return jsoncValue{}, fmt.Errorf("config: unterminated object")
		}
		if p.data[memberStart] == '}' {
			return jsoncValue{kind: valueObject, start: start, end: memberStart + 1}, nil
		}
		keyEnd, err := p.parseString(memberStart)
		if err != nil {
			return jsoncValue{}, err
		}
		colon, err := p.skipTrivia(keyEnd)
		if err != nil {
			return jsoncValue{}, err
		}
		if colon >= len(p.data) || p.data[colon] != ':' {
			return jsoncValue{}, fmt.Errorf("config: expected ':' after object key")
		}
		value, err := p.skipValue(colon + 1)
		if err != nil {
			return jsoncValue{}, err
		}
		i, err = p.skipTrivia(value.end)
		if err != nil {
			return jsoncValue{}, err
		}
		if i < len(p.data) && p.data[i] == ',' {
			i++
		}
	}
}

func (p *jsoncParser) skipArray(i int) (jsoncValue, error) {
	start := i
	i++
	for {
		next, err := p.skipTrivia(i)
		if err != nil {
			return jsoncValue{}, err
		}
		if next >= len(p.data) {
			return jsoncValue{}, fmt.Errorf("config: unterminated array")
		}
		if p.data[next] == ']' {
			return jsoncValue{kind: valueArray, start: start, end: next + 1}, nil
		}
		value, err := p.skipValue(next)
		if err != nil {
			return jsoncValue{}, err
		}
		i, err = p.skipTrivia(value.end)
		if err != nil {
			return jsoncValue{}, err
		}
		if i < len(p.data) && p.data[i] == ',' {
			i++
		}
	}
}

func (p *jsoncParser) parseString(i int) (int, error) {
	if i >= len(p.data) || p.data[i] != '"' {
		return 0, fmt.Errorf("config: expected string at byte %d", i)
	}
	i++
	for i < len(p.data) {
		switch p.data[i] {
		case '\\':
			i += 2
		case '"':
			return i + 1, nil
		default:
			i++
		}
	}
	return 0, fmt.Errorf("config: unterminated string")
}

func (p *jsoncParser) parseNumber(i int) (int, error) {
	start := i
	for i < len(p.data) {
		ch := p.data[i]
		if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' || ch == '.' || ch == 'e' || ch == 'E' {
			i++
			continue
		}
		break
	}
	if i == start {
		return 0, fmt.Errorf("config: invalid value at byte %d", start)
	}
	return i, nil
}

func (p *jsoncParser) parseLiteral(i int) (int, error) {
	for _, lit := range [][]byte{[]byte("true"), []byte("false"), []byte("null")} {
		if bytes.HasPrefix(p.data[i:], lit) {
			return i + len(lit), nil
		}
	}
	return 0, fmt.Errorf("config: invalid literal at byte %d", i)
}

func (p *jsoncParser) skipTrivia(i int) (int, error) {
	for i < len(p.data) {
		switch p.data[i] {
		case ' ', '\t', '\n', '\r':
			i++
		case '/':
			if i+1 >= len(p.data) {
				return 0, fmt.Errorf("config: invalid trailing slash")
			}
			switch p.data[i+1] {
			case '/':
				i += 2
				for i < len(p.data) && p.data[i] != '\n' {
					i++
				}
			case '*':
				i += 2
				for i+1 < len(p.data) && !(p.data[i] == '*' && p.data[i+1] == '/') {
					i++
				}
				if i+1 >= len(p.data) {
					return 0, fmt.Errorf("config: unterminated block comment")
				}
				i += 2
			default:
				return i, nil
			}
		default:
			return i, nil
		}
	}
	return i, nil
}
