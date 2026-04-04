package input_test

import (
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/input"
)

func TestBody_NoArgsNoStdin(t *testing.T) {
	body, err := input.Body(strings.NewReader(""), true, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != nil {
		t.Errorf("expected nil body with no args and TTY stdin, got %v", body)
	}
}

func TestBody_ShorthandArgs(t *testing.T) {
	// Simulate: restish post /url name: Alice, age: 30
	// Shell splits into tokens; we receive them already split.
	args := []string{"name:", "Alice,", "age:", "30"}
	body, err := input.Body(strings.NewReader(""), true, args, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T: %v", body, body)
	}
	if m["name"] != "Alice" {
		t.Errorf("name: got %v, want Alice", m["name"])
	}
	// shorthand may produce int, int64, or float64 depending on the value.
	switch v := m["age"].(type) {
	case int:
		if v != 30 {
			t.Errorf("age: got %v, want 30", v)
		}
	case int64:
		if v != 30 {
			t.Errorf("age: got %v, want 30", v)
		}
	case float64:
		if v != 30 {
			t.Errorf("age: got %v, want 30", v)
		}
	default:
		t.Errorf("age: got %T(%v), want numeric 30", m["age"], m["age"])
	}
}

func TestBody_NestedShorthand(t *testing.T) {
	args := []string{"user.address.city:", "NYC"}
	body, err := input.Body(strings.NewReader(""), true, args, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", body)
	}
	user, ok := m["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user map, got %T", m["user"])
	}
	addr, ok := user["address"].(map[string]any)
	if !ok {
		t.Fatalf("expected address map, got %T", user["address"])
	}
	if addr["city"] != "NYC" {
		t.Errorf("city: got %v, want NYC", addr["city"])
	}
}

func TestBody_StdinPassthrough(t *testing.T) {
	// Simulate piped stdin with no args — parsed into a Go value.
	body, err := input.Body(strings.NewReader(`{"piped":true}`), false, nil, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map from stdin JSON, got %T: %v", body, body)
	}
	if m["piped"] != true {
		t.Errorf("piped: got %v, want true", m["piped"])
	}
}

func TestBody_StdinPlusArgsPatch(t *testing.T) {
	// Stdin JSON is the base; shorthand args patch on top.
	args := []string{"name:", "Alice"}
	body, err := input.Body(strings.NewReader(`{"name":"Bob","age":25}`), false, args, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", body)
	}
	if m["name"] != "Alice" {
		t.Errorf("name: got %v, want Alice (patched)", m["name"])
	}
}

func TestBody_FormKeepsFileReferenceLiteral(t *testing.T) {
	args := []string{"file:", "@upload.txt"}
	body, err := input.Body(strings.NewReader(""), true, args, "form")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", body)
	}
	if got := m["file"]; got != "@upload.txt" {
		t.Fatalf("expected literal file reference, got %v", got)
	}
}
