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
		"links": map[string]any{
			"next": "/next",
		},
		"body": map[string]any{
			"id":   float64(42),
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

func TestAutoDetect_JQDotFieldDoesNotFallBackToShorthand(t *testing.T) {
	_, err := filter.Apply(".body.items[].{id}", testDoc(), filter.LangAuto)
	if err == nil {
		t.Fatal("expected jq parse error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "jq parse:") {
		t.Fatalf("expected jq parse error, got %q", msg)
	}
	if strings.Contains(msg, "shorthand:") {
		t.Fatalf("expected leading dot field to be treated as jq only, got %q", msg)
	}
}

func TestAutoDetect_JQBuiltin(t *testing.T) {
	// "length" has no shorthand root → jq
	_, err := filter.Apply("length", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAutoDetectTreatsAmbiguousRuntimeJQAsShorthand(t *testing.T) {
	result, err := filter.Apply("body.items[0].id", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != float64(1) {
		t.Fatalf("got %v, want 1", result)
	}
}

func TestAutoDetectShorthandProjectionObject(t *testing.T) {
	result, err := filter.Apply("{next: links.next, id: body.id}", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %T, want map", result)
	}
	if m["next"] != "/next" || m["id"] != float64(42) {
		t.Fatalf("result = %#v, want next and id projection", result)
	}
}

func TestAutoDetectJQObjectWithCurrentRoot(t *testing.T) {
	result, err := filter.Apply("{next: .links.next, id: .body.id}", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %T, want map", result)
	}
	if m["next"] != "/next" || m["id"] != float64(42) {
		t.Fatalf("result = %#v, want next and id projection", result)
	}
}

func TestAutoDetectShorthandRecursiveDescent(t *testing.T) {
	doc := testDoc()
	doc["body"].(map[string]any)["example"] = map[string]any{"foo": "found"}
	result, err := filter.Apply("..foo", doc, filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	values, ok := result.([]any)
	if !ok || len(values) != 1 || values[0] != "found" {
		t.Fatalf("result = %#v, want recursive shorthand match", result)
	}
}

func TestAutoDetectJQRecursiveDescentPipe(t *testing.T) {
	result, err := filter.Apply(".. | .id?", map[string]any{"id": "root"}, filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "root" {
		t.Fatalf("result = %#v, want root id from jq recursive descent", result)
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

func TestAutoDetectOrdersJQIntentErrorFirst(t *testing.T) {
	_, err := filter.Apply("{next: .links[}", testDoc(), filter.LangAuto)
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrorOrder(t, err.Error(), "jq parse:", "shorthand:")
}

func TestAutoDetectOrdersShorthandIntentErrorFirst(t *testing.T) {
	_, err := filter.Apply("{next: links[}", testDoc(), filter.LangAuto)
	if err == nil {
		t.Fatal("expected error")
	}
	assertErrorOrder(t, err.Error(), "shorthand:", "jq parse:")
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

func assertErrorOrder(t *testing.T, msg, first, second string) {
	t.Helper()
	firstIndex := strings.Index(msg, first)
	secondIndex := strings.Index(msg, second)
	if firstIndex < 0 || secondIndex < 0 {
		t.Fatalf("expected %q and %q in %q", first, second, msg)
	}
	if firstIndex > secondIndex {
		t.Fatalf("expected %q before %q in %q", first, second, msg)
	}
}

func TestHeaderFieldMissingFallsBackToFilterBackend(t *testing.T) {
	result, err := filter.Apply("headers.Missing", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("missing header = %#v, want nil", result)
	}
}

func TestHeaderFieldReadsMultiValueHeaderMap(t *testing.T) {
	doc := map[string]any{
		"headers": map[string][]string{"Set-Cookie": {"a=1", "b=2"}},
	}
	result, err := filter.Apply("headers.set-cookie", doc, filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "a=1,b=2" {
		t.Fatalf("header = %#v, want joined values", result)
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

func TestAutoDetectPrefersJQIntentMarkers(t *testing.T) {
	result, err := filter.Apply(".body.name", testDoc(), filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Alice" {
		t.Fatalf("auto jq result = %#v, want Alice", result)
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

func TestJQ_NormalizesTypedMapsAndSlices(t *testing.T) {
	doc := map[string]any{
		"headers": map[string]string{
			"X-Test-Header": "ok",
		},
		"headers_all": map[string][]string{
			"Set-Cookie": {"session=secret", "theme=light"},
		},
		"body": map[string][]string{
			"tags": {"one", "two"},
		},
	}

	result, err := filter.Apply(`{header: .headers["X-Test-Header"], cookie: .headers_all["Set-Cookie"][0], tags: .body.tags}`, doc, filter.LangJQ)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v (%T), want object", result, result)
	}
	if obj["header"] != "ok" || obj["cookie"] != "session=secret" {
		t.Fatalf("unexpected projected values: %#v", obj)
	}
	tags, ok := obj["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "one" || tags[1] != "two" {
		t.Fatalf("tags = %#v, want normalized string slice", obj["tags"])
	}
}

func TestAutoDetectJQProjectionNormalizesTypedHeaders(t *testing.T) {
	doc := map[string]any{
		"headers": map[string]string{
			"Date": "Mon, 02 Jan 2006 15:04:05 GMT",
		},
		"body": map[string]any{"name": "Alice"},
	}

	result, err := filter.Apply(`{date: .headers.Date, name: .body.name}`, doc, filter.LangAuto)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	obj, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v (%T), want object", result, result)
	}
	if obj["date"] != "Mon, 02 Jan 2006 15:04:05 GMT" || obj["name"] != "Alice" {
		t.Fatalf("unexpected projection: %#v", obj)
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
