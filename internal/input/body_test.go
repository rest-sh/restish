package input_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/input"
)

func TestBody_NoArgsNoStdin(t *testing.T) {
	body, err := input.Body(strings.NewReader(""), true, nil, "", input.BodyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != nil {
		t.Errorf("expected nil body with no args and TTY stdin, got %v", body)
	}
}

func TestBodyWarnsWhenUnstructuredStdinIsIgnoredForArgs(t *testing.T) {
	var warnings []string
	body, err := input.Body(strings.NewReader("plain text"), false, []string{"name:", "Ada"}, "", input.BodyOptions{
		Warnf: func(format string, args ...any) {
			warnings = append(warnings, format)
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want one", warnings)
	}
	m, ok := body.(map[string]any)
	if !ok || m["name"] != "Ada" {
		t.Fatalf("body = %#v, want args-only map", body)
	}
}

func TestBodyRejectsOversizedStdin(t *testing.T) {
	_, err := input.Body(bytes.NewReader(bytes.Repeat([]byte("x"), input.MaxStdinBodyBytes+1)), false, nil, "", input.BodyOptions{})
	if err == nil {
		t.Fatal("expected oversized stdin error")
	}
	if !strings.Contains(err.Error(), "stdin body exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBody_ShorthandArgs(t *testing.T) {
	// Simulate: restish post /url name: Alice, age: 30
	// Shell splits into tokens; we receive them already split.
	args := []string{"name:", "Alice,", "age:", "30"}
	body, err := input.Body(strings.NewReader(""), true, args, "", input.BodyOptions{})
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

func TestBody_ShorthandCommaSemantics(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want map[string]any
	}{
		{
			name: "split comma separated fields",
			args: []string{"name:", "Alice,", "enabled:", "true"},
			want: map[string]any{"name": "Alice", "enabled": true},
		},
		{
			name: "quoted comma separated fields",
			args: []string{"name: Alice, enabled: true"},
			want: map[string]any{"name": "Alice", "enabled": true},
		},
		{
			name: "single field value with spaces",
			args: []string{"note:", "Alice", "enabled:", "true"},
			want: map[string]any{"note": "Alice enabled: true"},
		},
		{
			name: "missing comma remains one string value",
			args: []string{"name:", "Alice", "enabled:", "true"},
			want: map[string]any{"name": "Alice enabled: true"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, err := input.Body(strings.NewReader(""), true, tc.args, "", input.BodyOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			m, ok := body.(map[string]any)
			if !ok {
				t.Fatalf("body = %T(%#v), want map", body, body)
			}
			for key, want := range tc.want {
				if got := m[key]; got != want {
					t.Fatalf("%s = %#v, want %#v; body=%#v", key, got, want, body)
				}
			}
			if len(m) != len(tc.want) {
				t.Fatalf("body = %#v, want keys %#v", body, tc.want)
			}
		})
	}
}

func TestBody_NestedShorthand(t *testing.T) {
	args := []string{"user.address.city:", "NYC"}
	body, err := input.Body(strings.NewReader(""), true, args, "", input.BodyOptions{})
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
	body, err := input.Body(strings.NewReader(`{"piped":true}`), false, nil, "", input.BodyOptions{})
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

func TestBody_StdinPlainTextPassthrough(t *testing.T) {
	body, err := input.Body(strings.NewReader("This is not JSON!"), false, nil, "", input.BodyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "This is not JSON!" {
		t.Fatalf("body = %#v, want plain text string", body)
	}
}

func TestBody_StdinPlusArgsPatch(t *testing.T) {
	// Stdin JSON is the base; shorthand args patch on top.
	args := []string{"name:", "Alice"}
	body, err := input.Body(strings.NewReader(`{"name":"Bob","age":25}`), false, args, "", input.BodyOptions{})
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
	body, err := input.Body(strings.NewReader(""), true, args, "form", input.BodyOptions{})
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
