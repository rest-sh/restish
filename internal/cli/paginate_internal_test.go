package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/config"
)

func TestMergePaginatedBodyPreservesWrapperObject(t *testing.T) {
	firstBody := map[string]any{
		"data": []any{
			map[string]any{"id": float64(1)},
			map[string]any{"id": float64(2)},
		},
		"meta": map[string]any{"source": "test"},
	}
	items := []any{
		map[string]any{"id": float64(1)},
		map[string]any{"id": float64(2)},
		map[string]any{"id": float64(3)},
		map[string]any{"id": float64(4)},
	}

	got := mergePaginatedBody(firstBody, &config.PaginationConfig{ItemsPath: "data"}, items)

	want := map[string]any{
		"data": items,
		"meta": map[string]any{"source": "test"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergePaginatedBody() = %#v, want %#v", got, want)
	}
}

func TestSetSimplePathNestedObject(t *testing.T) {
	value := map[string]any{
		"meta": map[string]any{
			"items": []any{1, 2},
		},
	}

	got, ok := setSimplePath(value, "meta.items", []any{1, 2, 3})
	if !ok {
		t.Fatal("expected nested simple path to be set")
	}

	want := map[string]any{
		"meta": map[string]any{
			"items": []any{1, 2, 3},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("setSimplePath() = %#v, want %#v", got, want)
	}
}

func TestPaginatedReadableFrameForRootArray(t *testing.T) {
	frame, ok := paginatedReadableFrame([]any{1, 2}, nil)
	if !ok {
		t.Fatal("expected frame for root array")
	}
	if frame.Prefix != "" {
		t.Fatalf("Prefix = %q, want empty", frame.Prefix)
	}
	if frame.Suffix != "" {
		t.Fatalf("Suffix = %q, want empty", frame.Suffix)
	}
	if frame.ItemIndent != "  " {
		t.Fatalf("ItemIndent = %q, want %q", frame.ItemIndent, "  ")
	}
	if frame.CloseIndent != "" {
		t.Fatalf("CloseIndent = %q, want empty", frame.CloseIndent)
	}
}

func TestPaginatedReadableFramePreservesWrapperObject(t *testing.T) {
	firstBody := map[string]any{
		"data": []any{
			map[string]any{"id": float64(1)},
		},
		"meta": map[string]any{"source": "test"},
	}
	frame, ok := paginatedReadableFrame(firstBody, &config.PaginationConfig{ItemsPath: "data"})
	if !ok {
		t.Fatal("expected frame for wrapped object")
	}
	if !strings.Contains(frame.Prefix, `"data": `) {
		t.Fatalf("expected Prefix to contain data field, got %q", frame.Prefix)
	}
	if !strings.Contains(frame.Suffix, `"meta": {`) {
		t.Fatalf("expected Suffix to contain meta field, got %q", frame.Suffix)
	}
	if frame.ItemIndent != "    " {
		t.Fatalf("ItemIndent = %q, want %q", frame.ItemIndent, "    ")
	}
	if frame.CloseIndent != "  " {
		t.Fatalf("CloseIndent = %q, want %q", frame.CloseIndent, "  ")
	}
}

func TestPaginationItemCapacity(t *testing.T) {
	tests := []struct {
		name           string
		firstPageItems int
		maxPages       int
		maxItems       int
		want           int
	}{
		{name: "uses first page without limits", firstPageItems: 3, want: 3},
		{name: "scales by max pages", firstPageItems: 3, maxPages: 4, want: 12},
		{name: "caps by max items", firstPageItems: 3, maxPages: 4, maxItems: 5, want: 5},
		{name: "returns max items when first page already exceeds it", firstPageItems: 10, maxItems: 5, want: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := paginationItemCapacity(tc.firstPageItems, tc.maxPages, tc.maxItems); got != tc.want {
				t.Fatalf("paginationItemCapacity() = %d, want %d", got, tc.want)
			}
		})
	}
}
