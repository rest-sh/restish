package auth

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestBearerAuthenticate(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	err := (&Bearer{}).Authenticate(context.Background(), req, AuthContext{Params: map[string]string{
		"token": "secret-token",
	}})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want bearer token", got)
	}
}

func TestBearerAuthenticateDoesNotOverwriteAuthorization(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	req.Header.Set("Authorization", "Bearer manual")
	err := (&Bearer{}).Authenticate(context.Background(), req, AuthContext{Params: map[string]string{
		"token": "configured",
	}})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer manual" {
		t.Fatalf("Authorization = %q, want manual value", got)
	}
}

func TestBearerAuthenticateRequiresToken(t *testing.T) {
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	err := (&Bearer{}).Authenticate(context.Background(), req, AuthContext{Params: map[string]string{}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "token is required") {
		t.Fatalf("error = %q, want token requirement", err)
	}
}
