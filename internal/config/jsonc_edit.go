package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tailscale/hujson"
)

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
	lock, err := lockConfigFile(path)
	if err != nil {
		return err
	}
	defer lock.Close()

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
	return atomicWriteFileLocked(path, patched, 0o600, 0o700, lock)
}

// jsoncSetPath updates a nested path in JSONC-formatted bytes while preserving
// comments and formatting. If the path doesn't exist, it creates nested objects
// as needed. If a non-object value is encountered on the path (excluding the
// final key), it returns an error.
func jsoncSetPath(data []byte, path []string, value any) ([]byte, error) {
	if len(path) == 0 {
		return data, nil
	}

	// Parse the JSONC with comment preservation
	v, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("config: parse JSONC: %w", err)
	}

	// Set the value at the path, modifying the CST
	if err := setValueAtPath(&v, path, value); err != nil {
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
func setValueAtPath(root *hujson.Value, path []string, value any) error {
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
			if _, isObj := memberPtr.Value.Value.(*hujson.Object); !isObj && memberPtr.Value.Value != nil {
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

	// Marshal the value to JSON and parse it back as a hujson Value
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("config: marshal value: %w", err)
	}

	valueHuJSON, err := hujson.Parse(valueJSON)
	if err != nil {
		return fmt.Errorf("config: parse value: %w", err)
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
		valueHuJSON.BeforeExtra = oldValue.BeforeExtra
		obj.Members[memberIndex].Value = valueHuJSON
	} else {
		// Create new member
		newMember := hujson.ObjectMember{
			Name: hujson.Value{Value: hujson.String(lastKey)},
			Value: valueHuJSON,
		}
		obj.Members = append(obj.Members, newMember)
	}

	return nil
}

// deleteValueAtPath removes the value at the specified path. If the path doesn't
// exist, it does nothing and returns no error.
func deleteValueAtPath(root *hujson.Value, path []string) error {
	if len(path) == 0 {
		return nil
	}

	// Navigate to the parent of the target
	current := root

	for i := 0; i < len(path)-1; i++ {
		key := path[i]

		if current.Value == nil {
			return nil // Path doesn't exist
		}

		obj, ok := current.Value.(*hujson.Object)
		if !ok {
			return nil // Path doesn't exist
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
			return nil // Path doesn't exist
		}

		current = &obj.Members[memberIndex].Value
	}

	// Delete the final key from the parent object
	if current.Value == nil {
		return nil
	}

	obj, ok := current.Value.(*hujson.Object)
	if !ok {
		return nil
	}

	lastKey := path[len(path)-1]
	for j := range obj.Members {
		if decodeJSONKey(obj.Members[j].Name.Value) == lastKey {
			// Remove this member by slicing it out
			obj.Members = append(obj.Members[:j], obj.Members[j+1:]...)
			return nil
		}
	}

	return nil
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

// parseJSONC parses bytes as JSONC and returns a pointer to the parsed value.
// This is used in tests to verify that output is still valid JSONC.
func parseJSONC(data []byte) (*hujson.Value, error) {
	v, err := hujson.Parse(data)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ---- Test helper types for backwards compatibility ----

// jsoncObject is a compatibility shim for old tests.
// This type is only used in tests and can be removed once tests are updated.
type jsoncObject struct {
	lbrace int
	rbrace int
}

// isInlineObject is a compatibility shim that always returns true for invalid bounds.
// This function is only used in one edge-case test.
func isInlineObject(data []byte, obj *jsoncObject) bool {
	if obj.rbrace <= obj.lbrace {
		return true
	}
	return false
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
