package main

import (
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/plugin"
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

func TestFormatCSVRejectsNonObjectRows(t *testing.T) {
	err := FormatCSV(&strings.Builder{}, []any{"oops"})
	if err == nil || !strings.Contains(err.Error(), "row 0") {
		t.Fatalf("expected row error, got %v", err)
	}
}

func TestFormatCSVRejectsScalar(t *testing.T) {
	err := FormatCSV(&strings.Builder{}, "oops")
	if err == nil || !strings.Contains(err.Error(), "object or array of objects") {
		t.Fatalf("expected object-or-array error, got %v", err)
	}
}

func TestCSVFormatterStreamWritesHeaderOnce(t *testing.T) {
	var out strings.Builder
	f := newCSVFormatter(&out)

	for _, req := range []plugin.FormatterRequest{
		{Event: "start"},
		{Event: "item", Response: plugin.FormatterResponse{Body: map[string]any{"id": float64(1), "name": "alpha"}}},
		{Event: "item", Response: plugin.FormatterResponse{Body: map[string]any{"id": float64(2), "name": "beta"}}},
		{Event: "end"},
	} {
		if err := f.Handle(req); err != nil {
			t.Fatalf("Handle(%q): %v", req.Event, err)
		}
	}

	want := strings.Join([]string{
		"id,name",
		"1,alpha",
		"2,beta",
		"",
	}, "\n")
	if got := out.String(); got != want {
		t.Fatalf("output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestCSVFormatterWarnsAndIgnoresLateColumns(t *testing.T) {
	var out, diag strings.Builder
	f := newCSVFormatterWithDiagnostics(&out, &diag)

	if err := f.Handle(plugin.FormatterRequest{
		Event:    "item",
		Response: plugin.FormatterResponse{Body: map[string]any{"id": float64(1)}},
	}); err != nil {
		t.Fatalf("first row: %v", err)
	}
	if err := f.Handle(plugin.FormatterRequest{
		Event:    "item",
		Response: plugin.FormatterResponse{Body: map[string]any{"id": float64(2), "name": "beta"}},
	}); err != nil {
		t.Fatalf("item with extra field: %v", err)
	}
	if err := f.Handle(plugin.FormatterRequest{Event: "end"}); err != nil {
		t.Fatalf("end: %v", err)
	}

	if got, want := out.String(), "id\n1\n2\n"; got != want {
		t.Fatalf("output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
	if got := diag.String(); !strings.Contains(got, "ignoring fields not present in header at row 2: name") {
		t.Fatalf("expected schema drift warning, got %q", got)
	}
}

func TestCSVFormatterPadsMissingFieldsInLaterRows(t *testing.T) {
	var out, diag strings.Builder
	f := newCSVFormatterWithDiagnostics(&out, &diag)

	if err := f.Handle(plugin.FormatterRequest{
		Event:    "start",
		Response: plugin.FormatterResponse{Body: map[string]any{"id": float64(1), "name": "first"}},
	}); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := f.Handle(plugin.FormatterRequest{
		Event:    "item",
		Response: plugin.FormatterResponse{Body: map[string]any{"id": float64(2)}},
	}); err != nil {
		t.Fatalf("item with missing field: %v", err)
	}
	if err := f.Handle(plugin.FormatterRequest{Event: "end"}); err != nil {
		t.Fatalf("end: %v", err)
	}

	if got, want := out.String(), "id,name\n1,first\n2,\n"; got != want {
		t.Fatalf("output mismatch:\nwant:\n%s\ngot:\n%s", want, got)
	}
	if diag.String() != "" {
		t.Fatalf("unexpected diagnostics: %q", diag.String())
	}
}
