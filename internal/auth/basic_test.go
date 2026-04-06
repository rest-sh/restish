package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"
)

func TestHTTPBasic_Parameters(t *testing.T) {
	h := &HTTPBasic{}
	params := h.Parameters()
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	names := map[string]bool{}
	for _, p := range params {
		names[p.Name] = true
	}
	if !names["username"] {
		t.Error("expected username param")
	}
	if !names["password"] {
		t.Error("expected password param")
	}
}

func TestHTTPBasic_OnRequest_WithPassword(t *testing.T) {
	h := &HTTPBasic{}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{
		"username": "alice",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := req.Header.Get("Authorization")
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("alice:secret"))
	if got != expected {
		t.Errorf("Authorization: got %q, want %q", got, expected)
	}
}

func TestHTTPBasic_OnRequest_MissingUsername(t *testing.T) {
	h := &HTTPBasic{}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{"password": "secret"})
	if err == nil {
		t.Error("expected error for missing username")
	}
}

func TestHTTPBasic_OnRequest_PromptedPassword(t *testing.T) {
	h := &HTTPBasic{
		Prompter: func(prompt string) (string, error) {
			return "prompted-secret", nil
		},
	}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{"username": "bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := req.Header.Get("Authorization")
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("bob:prompted-secret"))
	if got != expected {
		t.Errorf("Authorization: got %q, want %q", got, expected)
	}
}

func TestHTTPBasic_OnRequest_NoPrompter_NoPassword(t *testing.T) {
	h := &HTTPBasic{} // Prompter is nil
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{"username": "alice"})
	if err == nil {
		t.Error("expected error when no password and no prompter")
	}
}

func TestHTTPBasic_OnRequest_PromptError(t *testing.T) {
	h := &HTTPBasic{
		Prompter: func(prompt string) (string, error) {
			return "", fmt.Errorf("prompt cancelled")
		},
	}
	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	err := h.OnRequest(req, map[string]string{"username": "alice"})
	if err == nil {
		t.Error("expected error when prompter fails")
	}
}
