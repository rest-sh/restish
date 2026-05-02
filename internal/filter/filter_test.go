package filter_test

import (
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/filter"
)

// testDoc builds a representative normalised response map.
func testDoc() map[string]any {
	return map[string]any{
		"proto":  "HTTP/1.1",
		"status": 200,
		"headers": map[string]any{
			"Content-Type": "application/json",
		},
		"body": map[string]any{
			"name": "Alice",
			"age":  float64(30),
			"items": []any{
				map[string]any{"id": float64(1), "status": "active", "name": "foo"},
				map[string]any{"id": float64(2), "status": "inactive", "name": "bar"},
			},
		},
	}
}

// --- Auto-detection ---

func TestAutoDetect_ShorthandBody(t *testing.T) {
	result, err := filter.Apply("body.name", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Alice" {
		t.Errorf("got %v, want Alice", result)
	}
}

func TestAutoDetect_JQDot(t *testing.T) {
	// Starts with "." → jq
	result, err := filter.Apply(".body.name", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Alice" {
		t.Errorf("got %v, want Alice", result)
	}
}

func TestAutoDetect_JQBuiltin(t *testing.T) {
	// "length" has no shorthand root → jq
	_, err := filter.Apply("length", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAutoDetectFallsBackToShorthandWhenJQCannotParse(t *testing.T) {
	doc := testDoc()
	doc["body"].(map[string]any)["example"] = map[string]any{"url": "https://github.com/rest-sh/restish"}
	result, err := filter.Apply("..url|[@ contains github]", doc, filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected shorthand fallback result")
	}
}

func TestAutoDetectReturnsJQAndShorthandErrorsWhenBothFail(t *testing.T) {
	_, err := filter.Apply("..[", testDoc(), filter.LangAuto)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "jq parse:") {
		t.Fatalf("expected jq parse error, got %q", msg)
	}
	if !strings.Contains(msg, "shorthand:") {
		t.Fatalf("expected shorthand error, got %q", msg)
	}
}

// --- Shorthand ---

func TestShorthand_BodyName(t *testing.T) {
	result, err := filter.Apply("body.name", testDoc(), filter.LangShorthand)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Alice" {
		t.Errorf("got %v, want Alice", result)
	}
}

func TestShorthand_ArrayIndex(t *testing.T) {
	result, err := filter.Apply("body.items[0]", testDoc(), filter.LangShorthand)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["name"] != "foo" {
		t.Errorf("got %v, want foo", m["name"])
	}
}

func TestShorthand_PredicateFilter(t *testing.T) {
	result, err := filter.Apply("body.items[status == active].name", testDoc(), filter.LangShorthand)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Predicate on an array returns a slice of matches.
	items, ok := result.([]any)
	if !ok {
		// Single match may be unwrapped to a scalar.
		if result != "foo" {
			t.Errorf("got %v (%T), want foo", result, result)
		}
		return
	}
	if len(items) != 1 || items[0] != "foo" {
		t.Errorf("got %v, want [foo]", items)
	}
}

func TestShorthand_AtReturnsDoc(t *testing.T) {
	doc := testDoc()
	result, err := filter.Apply("@", doc, filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("expected non-nil result for @")
	}
}

// --- jq ---

func TestJQ_BodyName(t *testing.T) {
	result, err := filter.Apply(".body.name", testDoc(), filter.LangJQ)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Alice" {
		t.Errorf("got %v, want Alice", result)
	}
}

func TestJQ_SelectFilter(t *testing.T) {
	result, err := filter.Apply(`.body.items[] | select(.status == "active") | .name`, testDoc(), filter.LangJQ)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "foo" {
		t.Errorf("got %v, want foo", result)
	}
}

func TestJQForcedParseErrorDoesNotIncludeShorthandFallback(t *testing.T) {
	_, err := filter.Apply("..[", testDoc(), filter.LangJQ)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "jq parse:") {
		t.Fatalf("expected jq parse error, got %q", msg)
	}
	if strings.Contains(msg, "shorthand:") {
		t.Fatalf("forced jq should not include shorthand fallback error, got %q", msg)
	}
}
