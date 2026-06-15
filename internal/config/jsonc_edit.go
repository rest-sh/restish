package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/shorthand/v2"
	"github.com/rest-sh/restish/v2/config"
	"github.com/rest-sh/restish/v2/internal/fileutil"
	"github.com/tailscale/hujson"
	"github.com/tidwall/jsonc"
)

// ErrPathNotFound reports that a requested JSONC path did not exist.
var ErrPathNotFound = errors.New("config: path not found")

// ConfigPatchOperation describes one config edit operation.
// If Delete is true, Value is ignored and Path is removed.
type ConfigPatchOperation struct {
	Path   []string
	Value  any
	Delete bool
}

type jsoncFormatStyle struct {
	newline string
	indent  string
}

// SaveAPIConfig updates a single API entry in the JSONC config file while
// preserving surrounding comments and formatting when possible.
func SaveAPIConfig(path, apiName string, apiCfg *config.APIConfig) error {
	return patchConfig(path, true, func(data []byte) ([]byte, error) {
		return jsoncSetPath(data, []string{"apis", apiName}, apiCfg)
	})
}

// DeleteAPIConfig removes a single API entry from the JSONC config file while
// preserving surrounding comments and formatting when possible.
func DeleteAPIConfig(path, apiName string) error {
	return patchConfig(path, false, func(data []byte) ([]byte, error) {
		return jsoncDeletePath(data, []string{"apis", apiName})
	})
}

// SaveConfigValue updates a single object path inside the JSONC config file
// while preserving surrounding comments and formatting when possible.
func SaveConfigValue(path string, objectPath []string, value any) error {
	return patchConfig(path, true, func(data []byte) ([]byte, error) {
		return jsoncSetPath(data, objectPath, value)
	})
}

// SaveConfigValues applies multiple config edits atomically under one file lock,
// preserving JSONC comments and formatting where possible.
func SaveConfigValues(path string, ops []ConfigPatchOperation) error {
	if len(ops) == 0 {
		return nil
	}
	return patchConfig(path, true, func(data []byte) ([]byte, error) {
		var err error
		for _, op := range ops {
			if len(op.Path) == 0 {
				continue
			}
			if op.Delete {
				var next []byte
				next, err = jsoncDeletePath(data, op.Path)
				if errors.Is(err, ErrPathNotFound) {
					err = nil
					continue
				}
				data = next
			} else {
				data, err = jsoncSetPath(data, op.Path, op.Value)
			}
			if err != nil {
				return nil, err
			}
		}
		return data, nil
	})
}

// SaveConfigShorthand applies shorthand patch expressions to the JSONC config
// file under rootPath, validates the final config, and writes it atomically
// while preserving comments where possible.
func SaveConfigShorthand(path string, rootPath []string, exprs []string, validate func(*config.Config) error) error {
	if len(exprs) == 0 {
		return nil
	}
	if err := patchConfig(path, true, func(data []byte) ([]byte, error) {
		patched, cfg, err := PatchConfigShorthandBytes(data, rootPath, exprs)
		if err != nil {
			return nil, err
		}
		if validate != nil {
			if err := validate(cfg); err != nil {
				return nil, err
			}
		}
		return patched, nil
	}); err != nil {
		return err
	}
	return nil
}

// SaveConfigMutation applies a typed config mutation atomically under the
// config file lock while preserving JSONC comments and formatting where
// possible.
func SaveConfigMutation(path string, mutate func(*config.Config) error, validate func(*config.Config) error) error {
	return patchConfig(path, true, func(data []byte) ([]byte, error) {
		cfg, err := config.ParseConfigBytes(path, data)
		if err != nil {
			return nil, err
		}
		if mutate != nil {
			if err := mutate(cfg); err != nil {
				return nil, err
			}
		}
		if validate != nil {
			if err := validate(cfg); err != nil {
				return nil, err
			}
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			return nil, fmt.Errorf("config: marshal patched config: %w", err)
		}
		var root any
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("config: unmarshal patched config: %w", err)
		}
		return jsoncSyncGeneric(data, root)
	})
}

// PatchConfigShorthandBytes applies shorthand patch expressions to JSONC config
// bytes and returns the patched bytes plus the decoded typed config.
func PatchConfigShorthandBytes(data []byte, rootPath []string, exprs []string) ([]byte, *config.Config, error) {
	root, err := genericJSONC(data)
	if err != nil {
		return nil, nil, err
	}
	opts := shorthand.ParseOptions{
		EnableObjectDetection: true,
		ForceStringKeys:       true,
	}
	target := root
	if len(rootPath) > 0 {
		if existing, ok := genericValueAtPath(root, rootPath); ok {
			target = existing
		} else {
			target = map[string]any{}
		}
	}
	patchExpr := strings.Join(exprs, ", ")
	doc := shorthand.NewDocument(opts)
	if err := doc.Parse(patchExpr); err != nil {
		return nil, nil, fmt.Errorf("invalid shorthand patch %q: %w", patchExpr, err)
	}
	target, err = applyShorthandPatch(target, doc.Operations)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid shorthand patch %q: %w", patchExpr, err)
	}
	if len(rootPath) > 0 {
		root = genericSetPath(root, rootPath, target)
	}

	if err := ValidateShape(root); err != nil {
		return nil, nil, err
	}

	raw, err := json.Marshal(root)
	if err != nil {
		return nil, nil, fmt.Errorf("config: marshal patched config: %w", err)
	}
	cfg, err := config.ParseConfigBytes("", raw)
	if err != nil {
		return nil, nil, err
	}

	patched, err := jsoncSyncGeneric(data, root)
	if err != nil {
		return nil, nil, err
	}
	return patched, cfg, nil
}

type shorthandPathPart struct {
	key    string
	index  int
	isKey  bool
	append bool
	insert bool
}

func applyShorthandPatch(target any, ops []shorthand.Operation) (any, error) {
	var err error
	for _, op := range ops {
		switch op.Kind {
		case shorthand.OpSet:
			target, err = setShorthandPath(target, op.Path, op.Value)
		case shorthand.OpDelete:
			target, err = deleteShorthandPath(target, op.Path)
		case shorthand.OpSwap:
			rightPath, ok := op.Value.(string)
			if !ok {
				return nil, fmt.Errorf("swap operation value must be a path string, got %T", op.Value)
			}
			target, err = swapShorthandPaths(target, op.Path, rightPath)
		default:
			err = fmt.Errorf("unknown operation kind %d", op.Kind)
		}
		if err != nil {
			return nil, err
		}
	}
	return target, nil
}

func swapShorthandPaths(target any, leftPath, rightPath string) (any, error) {
	left, leftOK, err := shorthand.GetPath(leftPath, target, shorthand.GetOptions{})
	if err != nil {
		return nil, err
	}
	right, rightOK, err := shorthand.GetPath(rightPath, target, shorthand.GetOptions{})
	if err != nil {
		return nil, err
	}
	var applyErr error
	if rightOK {
		target, applyErr = setShorthandPath(target, leftPath, right)
	} else {
		target, applyErr = deleteShorthandPath(target, leftPath)
	}
	if applyErr != nil {
		return nil, applyErr
	}
	if leftOK {
		return setShorthandPath(target, rightPath, left)
	}
	return deleteShorthandPath(target, rightPath)
}

func setShorthandPath(root any, path string, value any) (any, error) {
	parts, err := parseShorthandPath(path)
	if err != nil {
		return nil, err
	}
	return setShorthandPathParts(root, parts, value)
}

func deleteShorthandPath(root any, path string) (any, error) {
	parts, err := parseShorthandPath(path)
	if err != nil {
		return nil, err
	}
	return deleteShorthandPathParts(root, parts)
}

func setShorthandPathParts(current any, parts []shorthandPathPart, value any) (any, error) {
	if len(parts) == 0 {
		return value, nil
	}
	part := parts[0]
	if part.isKey {
		m, ok := current.(map[string]any)
		if !ok {
			m = map[string]any{}
		}
		next, err := setShorthandPathParts(m[part.key], parts[1:], value)
		if err != nil {
			return nil, err
		}
		m[part.key] = next
		return m, nil
	}

	s, ok := current.([]any)
	if !ok {
		s = []any{}
	}
	index, err := resolveSetIndex(len(s), part)
	if err != nil {
		return nil, err
	}
	origLen := len(s)
	for len(s) <= index {
		s = append(s, nil)
	}
	if part.insert && index < origLen {
		s = append(s, nil)
		copy(s[index+1:], s[index:origLen])
	}
	next, err := setShorthandPathParts(s[index], parts[1:], value)
	if err != nil {
		return nil, err
	}
	s[index] = next
	return s, nil
}

func deleteShorthandPathParts(current any, parts []shorthandPathPart) (any, error) {
	if len(parts) == 0 {
		return current, nil
	}
	part := parts[0]
	if part.isKey {
		m, ok := current.(map[string]any)
		if !ok {
			return current, nil
		}
		if len(parts) == 1 {
			delete(m, part.key)
			return m, nil
		}
		child, ok := m[part.key]
		if !ok {
			return m, nil
		}
		next, err := deleteShorthandPathParts(child, parts[1:])
		if err != nil {
			return nil, err
		}
		m[part.key] = next
		return m, nil
	}

	s, ok := current.([]any)
	if !ok {
		return current, nil
	}
	if part.append {
		return s, nil
	}
	index := part.index
	if index < 0 {
		index = len(s) + index
	}
	if index < 0 || index >= len(s) {
		return s, nil
	}
	if len(parts) == 1 {
		return append(s[:index], s[index+1:]...), nil
	}
	next, err := deleteShorthandPathParts(s[index], parts[1:])
	if err != nil {
		return nil, err
	}
	s[index] = next
	return s, nil
}

func resolveSetIndex(length int, part shorthandPathPart) (int, error) {
	if part.append {
		return length, nil
	}
	index := part.index
	if index < 0 {
		index = length + index
	}
	if index < 0 {
		return 0, fmt.Errorf("index %d out of range", part.index)
	}
	return index, nil
}

func parseShorthandPath(path string) ([]shorthandPathPart, error) {
	var parts []shorthandPathPart
	for i := 0; i < len(path); {
		switch path[i] {
		case '.':
			i++
		case '[':
			part, next, err := parseShorthandIndex(path, i)
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
			i = next
		default:
			key, next, err := parseShorthandKey(path, i)
			if err != nil {
				return nil, err
			}
			if key == "" {
				return nil, fmt.Errorf("empty path component in %q", path)
			}
			parts = append(parts, shorthandPathPart{isKey: true, key: key})
			i = next
		}
	}
	return parts, nil
}

func parseShorthandIndex(path string, start int) (shorthandPathPart, int, error) {
	end := strings.IndexByte(path[start:], ']')
	if end < 0 {
		return shorthandPathPart{}, 0, fmt.Errorf("missing closing bracket in path %q", path)
	}
	content := path[start+1 : start+end]
	part := shorthandPathPart{}
	if content == "" {
		part.append = true
		return part, start + end + 1, nil
	}
	if strings.HasPrefix(content, "^") {
		part.insert = true
		content = strings.TrimPrefix(content, "^")
	}
	index, err := strconv.Atoi(content)
	if err != nil {
		return shorthandPathPart{}, 0, fmt.Errorf("invalid array index %q in path %q", content, path)
	}
	part.index = index
	return part, start + end + 1, nil
}

func parseShorthandKey(path string, start int) (string, int, error) {
	var b strings.Builder
	for i := start; i < len(path); i++ {
		switch path[i] {
		case '\\':
			if i+1 >= len(path) {
				return "", 0, fmt.Errorf("trailing escape in path %q", path)
			}
			b.WriteByte(path[i+1])
			i++
		case '"':
			i++
			for ; i < len(path); i++ {
				if path[i] == '\\' {
					if i+1 >= len(path) {
						return "", 0, fmt.Errorf("trailing escape in path %q", path)
					}
					b.WriteByte(path[i+1])
					i++
					continue
				}
				if path[i] == '"' {
					break
				}
				b.WriteByte(path[i])
			}
			if i >= len(path) || path[i] != '"' {
				return "", 0, fmt.Errorf("missing closing quote in path %q", path)
			}
		case '.', '[':
			return strings.TrimSpace(b.String()), i, nil
		default:
			b.WriteByte(path[i])
		}
	}
	return strings.TrimSpace(b.String()), len(path), nil
}

func genericValueAtPath(root any, path []string) (any, bool) {
	current := root
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[key]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func genericSetPath(root any, path []string, value any) any {
	if len(path) == 0 {
		return value
	}
	m, ok := root.(map[string]any)
	if !ok {
		m = map[string]any{}
	}
	child := genericSetPath(m[path[0]], path[1:], value)
	m[path[0]] = child
	return m
}

// ConfigShapeError reports one or more structural validation failures.
type ConfigShapeError struct {
	Errors []error
}

func (e *ConfigShapeError) Error() string {
	var b strings.Builder
	b.WriteString("config validation failed:")
	for _, err := range e.Errors {
		b.WriteString("\n  ")
		b.WriteString(formatHumaValidationError(err))
	}
	return b.String()
}

// ValidateShape validates generic decoded config data against the config.Config
// schema generated from Go structs.
func ValidateShape(value any) error {
	errs := huma.NewModelValidator().Validate(reflect.TypeOf(config.Config{}), value)
	if len(errs) == 0 {
		return nil
	}
	return &ConfigShapeError{Errors: errs}
}

func formatHumaValidationError(err error) string {
	var detailer huma.ErrorDetailer
	if errors.As(err, &detailer) {
		d := detailer.ErrorDetail()
		if d.Location != "" {
			return d.Location + ": " + d.Message
		}
		if d.Message != "" {
			return d.Message
		}
	}
	return err.Error()
}

func patchConfig(path string, createIfMissing bool, patch func([]byte) ([]byte, error)) error {
	lock, err := fileutil.LockSiblingFile(path)
	if err != nil {
		return err
	}
	defer lock.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if !createIfMissing {
				return nil
			}
			data = []byte("{}\n")
		} else {
			return fmt.Errorf("config: cannot read %s: %w", path, err)
		}
	}

	patched, err := patch(data)
	if err != nil {
		return err
	}
	if _, err := config.ParseConfigBytes(path, patched); err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path, patched, fileutil.AtomicWriteOptions{FileMode: 0o600, DirMode: 0o700})
}

func genericJSONC(data []byte) (any, error) {
	stripped := jsonc.ToJSON(data)
	if len(bytes.TrimSpace(stripped)) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(stripped))
	var root any
	if err := dec.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("config: parse JSONC: %w", err)
	}
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("unexpected trailing content")
		}
		return nil, fmt.Errorf("config: parse JSONC: %w", err)
	}
	if root == nil {
		root = map[string]any{}
	}
	return root, nil
}

func jsoncSyncGeneric(data []byte, value any) ([]byte, error) {
	v, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: parse JSONC: %w", err)
	}
	if err := syncHuJSONValue(&v, value); err != nil {
		return nil, err
	}
	return v.Pack(), nil
}

func syncHuJSONValue(dst *hujson.Value, value any) error {
	switch v := value.(type) {
	case map[string]any:
		if obj, ok := dst.Value.(*hujson.Object); ok {
			return syncHuJSONObject(obj, v)
		}
		return replaceHuJSONValue(dst, v)
	case []any:
		if arr, ok := dst.Value.(*hujson.Array); ok {
			return syncHuJSONArray(arr, v)
		}
		return replaceHuJSONValue(dst, v)
	default:
		return replaceHuJSONValue(dst, v)
	}
}

func syncHuJSONObject(obj *hujson.Object, value map[string]any) error {
	seen := map[string]bool{}
	out := obj.Members[:0]
	for i := range obj.Members {
		member := obj.Members[i]
		key := decodeJSONKey(member.Name.Value)
		next, ok := value[key]
		if !ok {
			continue
		}
		if err := syncHuJSONValue(&member.Value, next); err != nil {
			return err
		}
		seen[key] = true
		out = append(out, member)
	}
	obj.Members = out

	var added []string
	for key := range value {
		if !seen[key] {
			added = append(added, key)
		}
	}
	sort.Strings(added)
	for _, key := range added {
		val, err := genericHuJSONValue(value[key])
		if err != nil {
			return err
		}
		obj.Members = append(obj.Members, hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String(key)},
			Value: val,
		})
	}
	return nil
}

func syncHuJSONArray(arr *hujson.Array, value []any) error {
	n := len(value)
	if len(arr.Elements) > n {
		arr.Elements = arr.Elements[:n]
	}
	for i := 0; i < n; i++ {
		if i < len(arr.Elements) {
			if err := syncHuJSONValue(&arr.Elements[i], value[i]); err != nil {
				return err
			}
			continue
		}
		val, err := genericHuJSONValue(value[i])
		if err != nil {
			return err
		}
		arr.Elements = append(arr.Elements, val)
	}
	return nil
}

func replaceHuJSONValue(dst *hujson.Value, value any) error {
	next, err := genericHuJSONValue(value)
	if err != nil {
		return err
	}
	next.BeforeExtra = dst.BeforeExtra
	next.AfterExtra = dst.AfterExtra
	*dst = next
	return nil
}

func genericHuJSONValue(value any) (hujson.Value, error) {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return hujson.Value{}, fmt.Errorf("config: marshal value: %w", err)
	}
	valueHuJSON, err := hujson.Parse(valueJSON)
	if err != nil {
		return hujson.Value{}, fmt.Errorf("config: parse value: %w", err)
	}
	return valueHuJSON, nil
}

func formattedHuJSONValue(value any, depth int, style jsoncFormatStyle) (hujson.Value, error) {
	valueHuJSON, err := genericHuJSONValue(value)
	if err != nil {
		return hujson.Value{}, err
	}
	formatCompositeHuJSONValue(&valueHuJSON, depth, style)
	return valueHuJSON, nil
}

func formatCompositeHuJSONValue(value *hujson.Value, depth int, style jsoncFormatStyle) {
	switch v := value.Value.(type) {
	case *hujson.Object:
		if len(v.Members) == 0 {
			v.AfterExtra = nil
			return
		}
		for i := range v.Members {
			v.Members[i].Name.BeforeExtra = hujson.Extra(style.newline + strings.Repeat(style.indent, depth+1))
			v.Members[i].Name.AfterExtra = nil
			v.Members[i].Value.BeforeExtra = hujson.Extra(" ")
			v.Members[i].Value.AfterExtra = nil
			formatCompositeHuJSONValue(&v.Members[i].Value, depth+1, style)
		}
		v.AfterExtra = hujson.Extra(style.newline + strings.Repeat(style.indent, depth))
	case *hujson.Array:
		if len(v.Elements) == 0 {
			v.AfterExtra = nil
			return
		}
		for i := range v.Elements {
			v.Elements[i].BeforeExtra = hujson.Extra(style.newline + strings.Repeat(style.indent, depth+1))
			v.Elements[i].AfterExtra = nil
			formatCompositeHuJSONValue(&v.Elements[i], depth+1, style)
		}
		v.AfterExtra = hujson.Extra(style.newline + strings.Repeat(style.indent, depth))
	}
}

// jsoncSetPath updates a nested path in JSONC-formatted bytes while preserving
// comments and formatting. If the path doesn't exist, it creates nested objects
// as needed. If a non-object value is encountered on the path (excluding the
// final key), it returns an error.
func jsoncSetPath(data []byte, path []string, value any) ([]byte, error) {
	if len(path) == 0 {
		return data, nil
	}
	style := detectJSONCFormatStyle(data)

	// Parse the JSONC with comment preservation
	v, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: parse JSONC: %w", err)
	}

	// Set the value at the path, modifying the CST
	if err := setValueAtPath(&v, path, value, style); err != nil {
		return nil, err
	}

	// Pack back into bytes with comments preserved
	return v.Pack(), nil
}

// jsoncDeletePath removes a nested path from JSONC-formatted bytes while
// preserving surrounding comments.
func jsoncDeletePath(data []byte, path []string) ([]byte, error) {
	if len(path) == 0 {
		return data, nil
	}

	// Parse the JSONC with comment preservation
	v, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: parse JSONC: %w", err)
	}

	// Delete the value at the path
	if err := deleteValueAtPath(&v, path); err != nil {
		return nil, err
	}

	// Pack back into bytes
	return v.Pack(), nil
}

// setValueAtPath navigates to the specified path in the hujson tree and sets the value.
// Creates nested objects as needed. Returns an error if a non-object is encountered
// when trying to descend further.
func setValueAtPath(root *hujson.Value, path []string, value any, style jsoncFormatStyle) error {
	current := root

	// Navigate/create all but the last key
	for i := 0; i < len(path)-1; i++ {
		key := path[i]

		// Ensure current is an object
		if current.Value == nil {
			current.Value = &hujson.Object{}
		}

		obj, ok := current.Value.(*hujson.Object)
		if !ok {
			return fmt.Errorf(
				"cannot set nested path %q: value at %q is not an object (got %s)",
				strings.Join(path, "."),
				key,
				describeValueType(current.Value),
			)
		}

		// Find or create the key in the object
		var memberPtr *hujson.ObjectMember
		for j := range obj.Members {
			if decodeJSONKey(obj.Members[j].Name.Value) == key {
				memberPtr = &obj.Members[j]
				break
			}
		}

		if memberPtr == nil {
			// Create a new object member with a string key
			keyValue := hujson.Value{Value: hujson.String(key)}
			newValue := hujson.Value{
				Value: &hujson.Object{},
			}
			newMember := hujson.ObjectMember{
				Name:  keyValue,
				Value: newValue,
			}
			obj.Members = append(obj.Members, newMember)
			memberPtr = &obj.Members[len(obj.Members)-1]
		} else if i < len(path)-2 {
			// We found an existing member, but we still need to descend further
			// Check if it's an object
			if isJSONNull(memberPtr.Value.Value) {
				memberPtr.Value.Value = &hujson.Object{}
			} else if _, isObj := memberPtr.Value.Value.(*hujson.Object); !isObj && memberPtr.Value.Value != nil {
				// The value is not an object, error
				remainingPath := path[i:]
				return fmt.Errorf(
					"cannot set nested path %q: value at %q is not an object (got %s)",
					strings.Join(remainingPath, "."),
					key,
					describeValueType(memberPtr.Value.Value),
				)
			}
		}

		// Move into this member for the next iteration
		current = &memberPtr.Value
	}

	// Ensure we have an object at this level
	if current.Value == nil {
		current.Value = &hujson.Object{}
	} else if isJSONNull(current.Value) {
		current.Value = &hujson.Object{}
	}

	obj, ok := current.Value.(*hujson.Object)
	if !ok {
		return fmt.Errorf(
			"cannot set path %q: value is not an object (got %s)",
			strings.Join(path, "."),
			describeValueType(current.Value),
		)
	}

	// Set the final key
	lastKey := path[len(path)-1]

	valueHuJSON, err := formattedHuJSONValue(value, len(path), style)
	if err != nil {
		return err
	}

	// Find or create the member
	var memberIndex int = -1
	for j := range obj.Members {
		if decodeJSONKey(obj.Members[j].Name.Value) == lastKey {
			memberIndex = j
			break
		}
	}

	if memberIndex >= 0 {
		// Replace existing member's value while preserving BeforeExtra from the member
		oldValue := obj.Members[memberIndex].Value
		// Preserve the BeforeExtra of the old value (spacing before the value)
		if len(oldValue.BeforeExtra) > 0 {
			valueHuJSON.BeforeExtra = oldValue.BeforeExtra
		} else {
			valueHuJSON.BeforeExtra = hujson.Extra(" ")
		}
		obj.Members[memberIndex].Value = valueHuJSON
	} else {
		// Create new member
		name := hujson.Value{Value: hujson.String(lastKey)}
		name.BeforeExtra = hujson.Extra(style.newline + strings.Repeat(style.indent, len(path)))
		valueHuJSON.BeforeExtra = hujson.Extra(" ")
		newMember := hujson.ObjectMember{
			Name:  name,
			Value: valueHuJSON,
		}
		obj.Members = append(obj.Members, newMember)
		obj.AfterExtra = hujson.Extra(style.newline + strings.Repeat(style.indent, len(path)-1))
	}

	return nil
}

func detectJSONCFormatStyle(data []byte) jsoncFormatStyle {
	style := jsoncFormatStyle{newline: "\n", indent: "  "}
	if bytes.Contains(data, []byte("\r\n")) {
		style.newline = "\r\n"
	}
	lines := bytes.Split(data, []byte(style.newline))
	for _, line := range lines {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) == len(line) || len(trimmed) == 0 {
			continue
		}
		first := line[:len(line)-len(trimmed)]
		if len(first) > 0 {
			style.indent = string(first)
			break
		}
	}
	return style
}

func isJSONNull(value hujson.ValueTrimmed) bool {
	lit, ok := value.(hujson.Literal)
	return ok && string(lit) == "null"
}

// deleteValueAtPath removes the value at the specified path.
func deleteValueAtPath(root *hujson.Value, path []string) error {
	if len(path) == 0 {
		return nil
	}

	// Navigate to the parent of the target
	current := root

	for i := 0; i < len(path)-1; i++ {
		key := path[i]

		if current.Value == nil {
			return ErrPathNotFound
		}

		obj, ok := current.Value.(*hujson.Object)
		if !ok {
			return ErrPathNotFound
		}

		// Find the key in the object
		memberIndex := -1
		for j := range obj.Members {
			if decodeJSONKey(obj.Members[j].Name.Value) == key {
				memberIndex = j
				break
			}
		}

		if memberIndex < 0 {
			return ErrPathNotFound
		}

		current = &obj.Members[memberIndex].Value
	}

	// Delete the final key from the parent object
	if current.Value == nil {
		return ErrPathNotFound
	}

	obj, ok := current.Value.(*hujson.Object)
	if !ok {
		return ErrPathNotFound
	}

	lastKey := path[len(path)-1]
	for j := range obj.Members {
		if decodeJSONKey(obj.Members[j].Name.Value) == lastKey {
			// Remove this member by slicing it out
			obj.Members = append(obj.Members[:j], obj.Members[j+1:]...)
			return nil
		}
	}

	return ErrPathNotFound
}

// decodeJSONKey extracts the string value from a JSON-encoded key.
// For a hujson.String value, this decodes it to the actual string.
func decodeJSONKey(val hujson.ValueTrimmed) string {
	if lit, ok := val.(hujson.Literal); ok {
		// It's a literal (JSON string), try to unmarshal it
		var s string
		if err := json.Unmarshal(lit, &s); err == nil {
			return s
		}
		// Fallback: try to treat it as a raw string
		return string(lit)
	}
	return ""
}

// describeValueType returns a user-friendly description of a hujson value type.
func describeValueType(v hujson.ValueTrimmed) string {
	switch v.(type) {
	case *hujson.Object:
		return "object"
	case *hujson.Array:
		return "array"
	case hujson.Literal:
		return "literal"
	default:
		return fmt.Sprintf("%T", v)
	}
}
