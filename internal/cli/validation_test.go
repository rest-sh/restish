package cli

import (
	"strings"
	"testing"
)

func TestValidateGeneratedJSONBodyUsesSchemaDialectDefault(t *testing.T) {
	schema := map[string]any{
		"type":             "number",
		"minimum":          5.0,
		"exclusiveMinimum": true,
	}

	if err := validateGeneratedJSONBody(6.0, "application/json", "application/json", schema, "http://json-schema.org/draft-04/schema#", false); err != nil {
		t.Fatalf("draft-04 valid body: %v", err)
	}
	err := validateGeneratedJSONBody(5.0, "application/json", "application/json", schema, "http://json-schema.org/draft-04/schema#", false)
	if err == nil {
		t.Fatal("expected draft-04 exclusiveMinimum validation error")
	}
	if !strings.Contains(err.Error(), "request body failed OpenAPI schema validation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateGeneratedJSONBodySchemaKeywordOverridesDefaultDialect(t *testing.T) {
	schema := map[string]any{
		"$schema":          "http://json-schema.org/draft-04/schema#",
		"type":             "number",
		"minimum":          5.0,
		"exclusiveMinimum": true,
	}

	err := validateGeneratedJSONBody(5.0, "application/json", "application/json", schema, "https://json-schema.org/draft/2020-12/schema", false)
	if err == nil {
		t.Fatal("expected root $schema dialect to override default dialect")
	}
	if !strings.Contains(err.Error(), "request body failed OpenAPI schema validation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateGeneratedJSONBodyRejectsUnsupportedDefaultDialect(t *testing.T) {
	schema := map[string]any{"type": "object"}
	err := validateGeneratedJSONBody(map[string]any{}, "application/json", "application/json", schema, "https://schemas.example.com/future", false)
	if err == nil {
		t.Fatal("expected unsupported dialect error")
	}
	if !strings.Contains(err.Error(), `unsupported JSON Schema dialect "https://schemas.example.com/future"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateGeneratedJSONBodyFormatsAllLeafErrors(t *testing.T) {
	err := validateGeneratedJSONBody(map[string]any{
		"name": 123,
		"age":  "seventeen",
		"profile": map[string]any{
			"displayName": false,
			"settings": map[string]any{
				"newsletter": "yes",
				"theme":      "blue",
			},
		},
		"tags": []any{"ok", 42},
		"addresses": []any{
			map[string]any{"city": 99, "zip": "abc"},
			map[string]any{"zip": 90210},
		},
	}, "application/json", "application/json", validationFormatterSchema(), "https://spec.openapis.org/oas/3.1/dialect/base", false)
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	if strings.Contains(msg, "and ") {
		t.Fatalf("validation output should not truncate errors:\n%s", msg)
	}
	if strings.Contains(msg, "validation failed") {
		t.Fatalf("validation output should omit generic wrapper errors:\n%s", msg)
	}
	for _, want := range []string{
		"request body failed OpenAPI schema validation:\n",
		"\n  $.name: got number, want string",
		"\n  $.age: got string, want integer",
		"\n  $.profile.displayName: got boolean, want string",
		"\n  $.profile.settings.newsletter: got string, want boolean",
		"\n  $.profile.settings.theme: value must be one of 'light', 'dark'",
		"\n  $.tags[1]: got number, want string",
		"\n  $.addresses[0].city: got number, want string",
		"\n  $.addresses[0].zip: 'abc' does not match pattern '^[0-9]{5}$'",
		"\n  $.addresses[1]: missing property 'city'",
		"\n  $.addresses[1].zip: got number, want string",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("validation output missing %q:\n%s", want, msg)
		}
	}
}

func TestValidateGeneratedJSONBodyColorizesValidationOutput(t *testing.T) {
	err := validateGeneratedJSONBody(map[string]any{
		"name": 123,
	}, "application/json", "application/json", validationFormatterSchema(), "https://spec.openapis.org/oas/3.1/dialect/base", true)
	if err == nil {
		t.Fatal("expected validation error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "\x1b[") {
		t.Fatalf("expected ANSI color in validation output:\n%q", msg)
	}
	if plain := stripValidationANSI(msg); !strings.Contains(plain, "$.name: got number, want string") {
		t.Fatalf("colored validation output should preserve plain text after stripping ANSI:\n%s", plain)
	}
}

func stripValidationANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && (s[i] < 0x40 || s[i] > 0x7E) {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func validationFormatterSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []any{"name", "age", "profile", "tags", "addresses"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "integer"},
			"profile": map[string]any{
				"type":     "object",
				"required": []any{"displayName", "settings"},
				"properties": map[string]any{
					"displayName": map[string]any{"type": "string"},
					"settings": map[string]any{
						"type":     "object",
						"required": []any{"newsletter", "theme"},
						"properties": map[string]any{
							"newsletter": map[string]any{"type": "boolean"},
							"theme":      map[string]any{"enum": []any{"light", "dark"}},
						},
					},
				},
			},
			"tags": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"addresses": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"city", "zip"},
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
						"zip":  map[string]any{"type": "string", "pattern": "^[0-9]{5}$"},
					},
				},
			},
		},
	}
}
