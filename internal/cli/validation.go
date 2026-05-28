package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const generatedRequestSchemaURL = "restish-request-body.schema.json"

func validateGeneratedJSONBody(body any, contentType, schemaMediaType string, schema map[string]any, schemaDialect string, color bool) error {
	mediaType := strings.TrimSpace(contentType)
	if mediaType != "" && !strings.Contains(mediaType, "/") && schemaMediaType != "" {
		mediaType = schemaMediaType
	}
	if mediaType == "" {
		mediaType = schemaMediaType
	}
	if mediaType == "" {
		mediaType = "application/json"
	}
	if !isJSONMediaType(mediaType) {
		return fmt.Errorf("--rsh-validate only supports generated JSON request bodies; selected request content type is %s", mediaType)
	}
	if len(schema) == 0 {
		return fmt.Errorf("--rsh-validate requires a generated JSON request body schema for this operation")
	}
	normalizedSchema, err := normalizeJSONSchemaValue(schema)
	if err != nil {
		return fmt.Errorf("compile request body schema: %w", err)
	}
	rootSchemaDialect, _ := normalizedSchema["$schema"].(string)
	if shouldNormalizeOpenAPISchema(schemaDialect, rootSchemaDialect) {
		normalizeOpenAPIJSONSchema(normalizedSchema)
	}
	if isOpenAPIJSONSchemaDialect(rootSchemaDialect) {
		normalizedSchema["$schema"] = jsonschema.Draft2020.String()
	}
	defaultDraft, err := jsonSchemaDraftForDialect(schemaDialect)
	if err != nil && strings.TrimSpace(rootSchemaDialect) == "" {
		return fmt.Errorf("compile request body schema: %w", err)
	}
	if defaultDraft == nil {
		defaultDraft = jsonschema.Draft2020
	}

	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(defaultDraft)
	if err := compiler.AddResource(generatedRequestSchemaURL, normalizedSchema); err != nil {
		return fmt.Errorf("compile request body schema: %w", err)
	}
	compiled, err := compiler.Compile(generatedRequestSchemaURL)
	if err != nil {
		return fmt.Errorf("compile request body schema: %w", err)
	}
	if err := compiled.Validate(body); err != nil {
		prefix := "request body failed OpenAPI schema validation"
		if color {
			prefix = output.StyleText("diagnostic_error", prefix)
		}
		details := formatJSONSchemaValidationError(err, color)
		if strings.Contains(details, "\n") {
			return fmt.Errorf("%s:\n%s", prefix, details)
		}
		return fmt.Errorf("%s: %s", prefix, details)
	}
	return nil
}

func shouldNormalizeOpenAPISchema(defaultDialect, rootDialect string) bool {
	if isOpenAPIJSONSchemaDialect(defaultDialect) || isOpenAPIJSONSchemaDialect(rootDialect) {
		return true
	}
	return strings.TrimSpace(defaultDialect) == "" && strings.TrimSpace(rootDialect) == ""
}

func jsonSchemaDraftForDialect(dialect string) (*jsonschema.Draft, error) {
	switch normalizedJSONSchemaDialect(dialect) {
	case "":
		return jsonschema.Draft2020, nil
	case "https://json-schema.org/draft/2020-12/schema", "http://json-schema.org/draft/2020-12/schema":
		return jsonschema.Draft2020, nil
	case "https://json-schema.org/draft/2019-09/schema", "http://json-schema.org/draft/2019-09/schema":
		return jsonschema.Draft2019, nil
	case "https://json-schema.org/draft-07/schema", "http://json-schema.org/draft-07/schema":
		return jsonschema.Draft7, nil
	case "https://json-schema.org/draft-06/schema", "http://json-schema.org/draft-06/schema":
		return jsonschema.Draft6, nil
	case "https://json-schema.org/draft-04/schema", "http://json-schema.org/draft-04/schema":
		return jsonschema.Draft4, nil
	case "https://spec.openapis.org/oas/3.1/dialect/base",
		"https://spec.openapis.org/oas/3.2/dialect/2025-09-17":
		return jsonschema.Draft2020, nil
	default:
		return nil, fmt.Errorf("unsupported JSON Schema dialect %q", dialect)
	}
}

func normalizedJSONSchemaDialect(dialect string) string {
	dialect = strings.TrimSpace(dialect)
	if strings.HasSuffix(dialect, "#") {
		dialect = strings.TrimSuffix(dialect, "#")
	}
	return dialect
}

func isOpenAPIJSONSchemaDialect(dialect string) bool {
	switch normalizedJSONSchemaDialect(dialect) {
	case "https://spec.openapis.org/oas/3.1/dialect/base",
		"https://spec.openapis.org/oas/3.2/dialect/2025-09-17":
		return true
	default:
		return false
	}
}

func normalizeJSONSchemaValue(schema map[string]any) (map[string]any, error) {
	normalized, err := normalizeJSONCompatible(schema)
	if err != nil {
		return nil, err
	}
	out, ok := normalized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema root is %T, want object", normalized)
	}
	return out, nil
}

func normalizeJSONCompatible(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			normalized, err := normalizeJSONCompatible(child)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case map[interface{}]interface{}:
		out := make(map[string]any, len(v))
		for key, child := range v {
			keyString, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("schema object key is %T, want string", key)
			}
			normalized, err := normalizeJSONCompatible(child)
			if err != nil {
				return nil, err
			}
			out[keyString] = normalized
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			normalized, err := normalizeJSONCompatible(child)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	default:
		return v, nil
	}
}

func normalizeOpenAPIJSONSchema(value any) {
	switch v := value.(type) {
	case map[string]any:
		normalizeOpenAPIReadOnlyRequired(v)
		normalizeOpenAPIJSONSchemaMap(v)
		normalizeOpenAPIExclusiveBound(v, "minimum", "exclusiveMinimum")
		normalizeOpenAPIExclusiveBound(v, "maximum", "exclusiveMaximum")
		for _, child := range v {
			normalizeOpenAPIJSONSchema(child)
		}
	case []any:
		for _, child := range v {
			normalizeOpenAPIJSONSchema(child)
		}
	}
}

func normalizeOpenAPIJSONSchemaMap(schema map[string]any) {
	nullable, ok := schema["nullable"].(bool)
	if !ok || !nullable {
		return
	}
	switch typ := schema["type"].(type) {
	case string:
		if typ != "null" {
			schema["type"] = []any{typ, "null"}
		}
	case []any:
		for _, item := range typ {
			if item == "null" {
				return
			}
		}
		schema["type"] = append(typ, "null")
	case nil:
		withoutNullable := make(map[string]any, len(schema))
		for key, value := range schema {
			if key != "nullable" {
				withoutNullable[key] = value
			}
		}
		for key := range schema {
			delete(schema, key)
		}
		schema["anyOf"] = []any{withoutNullable, map[string]any{"type": "null"}}
	}
}

func normalizeOpenAPIReadOnlyRequired(schema map[string]any) {
	required, ok := schema["required"].([]any)
	if !ok || len(required) == 0 {
		return
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || len(properties) == 0 {
		return
	}
	out := required[:0]
	for _, item := range required {
		name, ok := item.(string)
		if !ok || !openAPIPropertyReadOnly(properties[name]) {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		delete(schema, "required")
		return
	}
	schema["required"] = out
}

func openAPIPropertyReadOnly(property any) bool {
	prop, ok := property.(map[string]any)
	if !ok {
		return false
	}
	readOnly, _ := prop["readOnly"].(bool)
	return readOnly
}

func normalizeOpenAPIExclusiveBound(schema map[string]any, limitKey, exclusiveKey string) {
	exclusive, ok := schema[exclusiveKey].(bool)
	if !ok {
		return
	}
	if !exclusive {
		delete(schema, exclusiveKey)
		return
	}
	if limit, ok := schema[limitKey]; ok {
		schema[exclusiveKey] = limit
	}
}

func formatJSONSchemaValidationError(err error, color bool) string {
	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return err.Error()
	}
	units := validationOutputLeaves(validationErr.BasicOutput())
	if len(units) == 0 {
		return validationErr.Error()
	}
	parts := validationOutputParts(units, color)
	if len(parts) == 0 {
		return validationErr.Error()
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "  " + strings.Join(parts, "\n  ")
}

func validationOutputParts(units []jsonschema.OutputUnit, color bool) []string {
	type validationOutputPart struct {
		text    string
		generic bool
	}
	var parts []validationOutputPart
	for _, unit := range units {
		msg := validationOutputMessage(unit.Error)
		if msg == "" {
			continue
		}
		generic := msg == "validation failed"
		path := jsonPointerDisplayPath(unit.InstanceLocation)
		if color {
			path = output.StyleText("key", path)
			msg = output.StyleText("diagnostic_error", msg)
		}
		parts = append(parts, validationOutputPart{
			text:    path + ": " + msg,
			generic: generic,
		})
	}
	if len(parts) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.generic {
			continue
		}
		filtered = append(filtered, part.text)
	}
	if len(filtered) > 0 {
		return filtered
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, part.text)
	}
	return out
}

func validationOutputLeaves(unit *jsonschema.OutputUnit) []jsonschema.OutputUnit {
	if unit == nil {
		return nil
	}
	if len(unit.Errors) == 0 {
		return []jsonschema.OutputUnit{*unit}
	}
	var out []jsonschema.OutputUnit
	for _, child := range unit.Errors {
		out = append(out, validationOutputLeaves(&child)...)
	}
	return out
}

func validationOutputMessage(err *jsonschema.OutputError) string {
	if err == nil {
		return ""
	}
	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		return ""
	}
	var msg string
	if unmarshalErr := json.Unmarshal(data, &msg); unmarshalErr != nil {
		return strings.Trim(string(data), `"`)
	}
	return msg
}

func jsonPointerDisplayPath(ptr string) string {
	if ptr == "" {
		return "$"
	}
	parts := strings.Split(strings.TrimPrefix(ptr, "/"), "/")
	var b strings.Builder
	b.WriteByte('$')
	for _, part := range parts {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		if part == "" {
			b.WriteString(`[""]`)
			continue
		}
		if _, err := strconv.Atoi(part); err == nil {
			b.WriteByte('[')
			b.WriteString(part)
			b.WriteByte(']')
			continue
		}
		if isIdentifierPathSegment(part) {
			b.WriteByte('.')
			b.WriteString(part)
			continue
		}
		encoded, _ := json.Marshal(part)
		b.WriteByte('[')
		b.Write(encoded)
		b.WriteByte(']')
	}
	return b.String()
}

func isIdentifierPathSegment(value string) bool {
	for i, r := range value {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return value != ""
}
