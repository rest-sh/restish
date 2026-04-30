package cli

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestValueStreamBaseForExplicitFilterOmitsResponsePreamble(t *testing.T) {
	base := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  http.StatusOK,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Links:   map[string]any{"next": "https://api.example.com/items?page=2"},
		Body:    []any{float64(1)},
	}

	got := valueStreamBaseForFilter(base, GlobalFlags{Filter: "body"})
	if got == base {
		t.Fatal("expected filtered value stream base to be copied")
	}
	if got.Proto != "" || got.Status != 0 || len(got.Headers) != 0 {
		t.Fatalf("filtered base kept response preamble fields: %#v", got)
	}
	if !reflect.DeepEqual(got.Body, base.Body) || !reflect.DeepEqual(got.Links, base.Links) {
		t.Fatalf("filtered base should preserve value context, got %#v want body=%#v links=%#v", got, base.Body, base.Links)
	}
	if base.Proto == "" || base.Status == 0 || len(base.Headers) == 0 {
		t.Fatalf("original base was mutated: %#v", base)
	}
}

func TestValueStreamBaseWithoutExplicitFilterKeepsResponsePreamble(t *testing.T) {
	base := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  http.StatusOK,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []any{float64(1)},
	}

	if got := valueStreamBaseForFilter(base, GlobalFlags{}); got != base {
		t.Fatalf("expected unfiltered value stream base to be preserved, got %#v", got)
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

func TestRunPaginationHonorsContextCancellation(t *testing.T) {
	c := New()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := &cobra.Command{}
	c.addGlobalFlags(cmd)
	cmd.SetContext(ctx)

	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})

	firstReq, err := http.NewRequest(http.MethodGet, "https://api.example.com/items", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	firstResp := &output.Response{
		Proto:   "HTTP/1.1",
		Status:  http.StatusOK,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Links:   map[string]any{"next": "https://api.example.com/items?page=2"},
		Body:    []any{float64(1)},
	}
	_ = firstReq
	go cancel()

	err = c.runPagination(cmd, firstResp, firstReq.URL.String(), "https://api.example.com/items?page=2", request.Options{
		Transport: request.BuildTransport(request.Options{Transport: c.baseHTTPTransport()}),
	}, nil, false, 25, 0)
	if err == nil {
		t.Fatal("expected pagination to stop on context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(..., %v)", err, context.Canceled)
	}
}
