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

	if err := validateGeneratedJSONBody(6.0, "application/json", "application/json", schema, "http://json-schema.org/draft-04/schema#"); err != nil {
		t.Fatalf("draft-04 valid body: %v", err)
	}
	err := validateGeneratedJSONBody(5.0, "application/json", "application/json", schema, "http://json-schema.org/draft-04/schema#")
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

	err := validateGeneratedJSONBody(5.0, "application/json", "application/json", schema, "https://json-schema.org/draft/2020-12/schema")
	if err == nil {
		t.Fatal("expected root $schema dialect to override default dialect")
	}
	if !strings.Contains(err.Error(), "request body failed OpenAPI schema validation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateGeneratedJSONBodyRejectsUnsupportedDefaultDialect(t *testing.T) {
	schema := map[string]any{"type": "object"}
	err := validateGeneratedJSONBody(map[string]any{}, "application/json", "application/json", schema, "https://schemas.example.com/future")
	if err == nil {
		t.Fatal("expected unsupported dialect error")
	}
	if !strings.Contains(err.Error(), `unsupported JSON Schema dialect "https://schemas.example.com/future"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
