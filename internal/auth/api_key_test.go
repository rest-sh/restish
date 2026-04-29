package auth

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestAPIKeyParameters(t *testing.T) {
	h := &APIKey{}
	params := h.Parameters()
	if len(params) != 3 {
		t.Fatalf("len(params) = %d, want 3", len(params))
	}
	seen := map[string]Param{}
	for _, p := range params {
		seen[p.Name] = p
	}
	for _, name := range []string{"in", "name", "value"} {
		if _, ok := seen[name]; !ok {
			t.Fatalf("missing parameter %q", name)
		}
	}
	if !seen["value"].Secret {
		t.Fatal("value parameter should be marked secret")
	}
}

func TestAPIKeyAuthenticateHeader(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	err := (&APIKey{}).Authenticate(context.Background(), req, AuthContext{Params: map[string]string{
		"in":    "header",
		"name":  "X-API-Key",
		"value": "secret-key",
	}})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got := req.Header.Get("X-API-Key"); got != "secret-key" {
		t.Fatalf("X-API-Key = %q, want secret-key", got)
	}
}

func TestAPIKeyAuthenticateQuery(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/items?page=1", nil)
	err := (&APIKey{}).Authenticate(context.Background(), req, AuthContext{Params: map[string]string{
		"in":    "query",
		"name":  "api_key",
		"value": "secret-key",
	}})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got := req.URL.Query().Get("api_key"); got != "secret-key" {
		t.Fatalf("api_key = %q, want secret-key", got)
	}
	if got := req.URL.Query().Get("page"); got != "1" {
		t.Fatalf("page = %q, want 1", got)
	}
}

func TestAPIKeyAuthenticateCookie(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	err := (&APIKey{}).Authenticate(context.Background(), req, AuthContext{Params: map[string]string{
		"in":    "cookie",
		"name":  "session_key",
		"value": "secret-key",
	}})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	cookie, err := req.Cookie("session_key")
	if err != nil {
		t.Fatalf("Cookie: %v", err)
	}
	if cookie.Value != "secret-key" {
		t.Fatalf("cookie value = %q, want secret-key", cookie.Value)
	}
}

func TestAPIKeyAuthenticateValidation(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]string
		want   string
	}{
		{name: "missing in", params: map[string]string{"name": "X-API-Key", "value": "secret"}, want: "in is required"},
		{name: "missing name", params: map[string]string{"in": "header", "value": "secret"}, want: "name is required"},
		{name: "missing value", params: map[string]string{"in": "header", "name": "X-API-Key"}, want: "value is required"},
		{name: "unsupported in", params: map[string]string{"in": "body", "name": "key", "value": "secret"}, want: "unsupported in"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
			err := (&APIKey{}).Authenticate(context.Background(), req, AuthContext{Params: tt.params})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}
