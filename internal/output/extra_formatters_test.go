package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/output"
)

func tableResp() *output.Response {
	return &output.Response{
		Proto:  "HTTP/1.1",
		Status: 200,
		Body: []any{
			map[string]any{"id": float64(1), "name": "Alice", "status": "active"},
			map[string]any{"id": float64(2), "name": "Bob", "status": "inactive"},
			map[string]any{"id": float64(3), "name": "Carol", "status": "active"},
		},
	}
}

// TestTableFormatter verifies that a 3-item array produces a Unicode table
// containing all values.
func TestTableFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &output.TableFormatter{}
	if err := f.Format(&buf, tableResp(), false); err != nil {
		t.Fatalf("Format: %v", err)
	}
	got := buf.String()

	// Border characters should be present.
	if !strings.Contains(got, "┌") || !strings.Contains(got, "┘") {
		t.Errorf("expected Unicode box drawing, got:\n%s", got)
	}
	// All names should appear.
	for _, name := range []string{"Alice", "Bob", "Carol"} {
		if !strings.Contains(got, name) {
			t.Errorf("expected %q in table output, got:\n%s", name, got)
		}
	}
	// Column headers should be present.
	for _, col := range []string{"id", "name", "status"} {
		if !strings.Contains(got, col) {
			t.Errorf("expected column header %q, got:\n%s", col, got)
		}
	}
}

// TestTableFormatterColumns verifies that --rsh-columns restricts the output.
func TestTableFormatterColumns(t *testing.T) {
	var buf bytes.Buffer
	f := &output.TableFormatter{Columns: []string{"name", "status"}}
	if err := f.Format(&buf, tableResp(), false); err != nil {
		t.Fatalf("Format: %v", err)
	}
	got := buf.String()

	if !strings.Contains(got, "name") || !strings.Contains(got, "status") {
		t.Errorf("expected name/status columns, got:\n%s", got)
	}
	// "id" column must not appear.
	// Note: check for column header only (values like "1" are too generic).
	lines := strings.Split(got, "\n")
	headerLine := lines[1] // second line is the header row
	if strings.Contains(headerLine, " id ") {
		t.Errorf("id column should be excluded, got header:\n%s", headerLine)
	}
}

// TestGronFormatter verifies that a nested object is rendered as gron paths.
func TestGronFormatter(t *testing.T) {
	resp := &output.Response{
		Status: 200,
		Body: map[string]any{
			"user": map[string]any{
				"name": "Alice",
				"age":  float64(30),
			},
			"tags": []any{"go", "cli"},
		},
	}
	var buf bytes.Buffer
	f := &output.GronFormatter{}
	if err := f.Format(&buf, resp, false); err != nil {
		t.Fatalf("Format: %v", err)
	}
	got := buf.String()

	expected := []string{
		`json = {};`,
		`json.tags = [];`,
		`json.tags[0] = "go";`,
		`json.tags[1] = "cli";`,
		`json.user = {};`,
		`json.user.age = 30;`,
		`json.user.name = "Alice";`,
	}
	for _, want := range expected {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in gron output, got:\n%s", want, got)
		}
	}
}
