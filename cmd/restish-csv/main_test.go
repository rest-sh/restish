package main

import (
	"strings"
	"testing"
)

func TestFormatCSV(t *testing.T) {
	var out strings.Builder
	body := []any{
		map[string]any{
			"id":    float64(1),
			"name":  "alpha",
			"tags":  []any{"x", "y"},
			"extra": map[string]any{"enabled": true},
		},
		map[string]any{
			"id":   float64(2),
			"name": "beta",
		},
	}

	if err := FormatCSV(&out, body); err != nil {
		t.Fatalf("FormatCSV: %v", err)
	}

	want := strings.Join([]string{
		"extra,id,name,tags",
		"\"{\"\"enabled\"\":true}\",1,alpha,\"[\"\"x\"\",\"\"y\"\"]\"",
		",2,beta,",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestFormatCSVRejectsNonArray(t *testing.T) {
	err := FormatCSV(&strings.Builder{}, map[string]any{"id": 1})
	if err == nil || !strings.Contains(err.Error(), "array of objects") {
		t.Fatalf("expected array-of-objects error, got %v", err)
	}
}

func TestFormatCSVRejectsNonObjectRows(t *testing.T) {
	err := FormatCSV(&strings.Builder{}, []any{"oops"})
	if err == nil || !strings.Contains(err.Error(), "row 0") {
		t.Fatalf("expected row error, got %v", err)
	}
}
