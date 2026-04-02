package request_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/request"
)

func TestDo_BasicGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.WriteHeader(200)
		_, _ = io.WriteString(w, "hello")
	}))
	t.Cleanup(srv.Close)

	resp, err := request.Do(context.Background(), "GET", srv.URL, nil, request.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("expected body 'hello', got %q", body)
	}
}

func TestDo_Headers(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Custom")
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	_, err := request.Do(context.Background(), "GET", srv.URL, nil, request.Options{
		Headers: []string{"X-Custom: test-value"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "test-value" {
		t.Errorf("expected X-Custom=test-value, got %q", gotHeader)
	}
}

func TestDo_HeaderWithColonInValue(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	// The value itself contains a colon; only the first colon separates name from value.
	_, err := request.Do(context.Background(), "GET", srv.URL, nil, request.Options{
		Headers: []string{"Authorization: Bearer tok:en"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "Bearer tok:en" {
		t.Errorf("expected 'Bearer tok:en', got %q", gotHeader)
	}
}

func TestDo_InvalidHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	_, err := request.Do(context.Background(), "GET", srv.URL, nil, request.Options{
		Headers: []string{"no-colon-at-all"},
	})
	if err == nil {
		t.Fatal("expected error for malformed header, got nil")
	}
}

func TestDo_Query(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("page")
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	_, err := request.Do(context.Background(), "GET", srv.URL, nil, request.Options{
		Query: []string{"page=2"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "2" {
		t.Errorf("expected page=2, got %q", gotQuery)
	}
}

func TestDo_InvalidQueryParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	_, err := request.Do(context.Background(), "GET", srv.URL, nil, request.Options{
		Query: []string{"no-equals-sign"},
	})
	if err == nil {
		t.Fatal("expected error for malformed query param, got nil")
	}
}

func TestDo_Post_WithBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(201)
	}))
	t.Cleanup(srv.Close)

	body := strings.NewReader(`{"name":"test"}`)
	resp, err := request.Do(context.Background(), "POST", srv.URL, body, request.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	if gotBody != `{"name":"test"}` {
		t.Errorf("unexpected body: %q", gotBody)
	}
}
