package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestValidateOIDCEndpoints(t *testing.T) {
	cases := []struct {
		name      string
		issuer    string
		authURL   string
		tokenURL  string
		wantError bool
	}{
		{
			name:      "valid same hostname",
			issuer:    "https://auth.example.com",
			authURL:   "https://auth.example.com/authorize",
			tokenURL:  "https://auth.example.com/token",
			wantError: false,
		},
		{
			name:      "attacker token endpoint",
			issuer:    "https://auth.example.com",
			authURL:   "https://auth.example.com/authorize",
			tokenURL:  "https://attacker.com/steal",
			wantError: true,
		},
		{
			name:      "http token endpoint with https issuer",
			issuer:    "https://auth.example.com",
			tokenURL:  "http://auth.example.com/token",
			wantError: true,
		},
		{
			name:      "http issuer skips validation",
			issuer:    "http://localhost:8080",
			authURL:   "http://localhost:8080/authorize",
			tokenURL:  "http://localhost:8080/token",
			wantError: false,
		},
		{
			name:      "empty endpoints are allowed",
			issuer:    "https://auth.example.com",
			authURL:   "",
			tokenURL:  "",
			wantError: false,
		},
		{
			name:      "case-insensitive hostname match",
			issuer:    "https://Auth.Example.COM",
			tokenURL:  "https://auth.example.com/token",
			wantError: false,
		},
		{
			name:      "idna hostname match",
			issuer:    "https://bücher.example",
			tokenURL:  "https://xn--bcher-kva.example/token",
			wantError: false,
		},
		{
			name:      "path-scoped issuer allows child endpoint paths",
			issuer:    "https://auth.example.com/realms/demo",
			authURL:   "https://auth.example.com/realms/demo/protocol/openid-connect/auth",
			tokenURL:  "https://auth.example.com/realms/demo/protocol/openid-connect/token",
			wantError: false,
		},
		{
			name:      "path-scoped issuer rejects sibling tenant endpoint paths",
			issuer:    "https://auth.example.com/realms/demo",
			tokenURL:  "https://auth.example.com/realms/other/protocol/openid-connect/token",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &OIDCConfig{
				AuthorizationEndpoint: tc.authURL,
				TokenEndpoint:         tc.tokenURL,
			}
			err := validateOIDCEndpoints(tc.issuer, cfg)
			if tc.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseTokenEndpointErrorRedactsSecrets(t *testing.T) {
	err := parseTokenEndpointError(400, []byte(`{
		"error":"invalid_client",
		"client_secret":"top-secret",
		"refresh_token":"refresh-secret",
		"details":{"access_token":"abc"}
	}`))
	var tokenErr *tokenEndpointError
	if !strings.Contains(err.Error(), "invalid_client") {
		t.Fatalf("expected error code in message, got %v", err)
	}
	if !errorAsToken(err, &tokenErr) {
		t.Fatalf("expected token endpoint error, got %T", err)
	}
	for _, secret := range []string{"top-secret", "refresh-secret", "abc"} {
		if strings.Contains(tokenErr.Body, secret) {
			t.Fatalf("expected body redaction, got %q", tokenErr.Body)
		}
	}
}

func TestValidateDirectOAuthEndpoint(t *testing.T) {
	cases := []struct {
		name      string
		param     string
		rawURL    string
		wantError bool
	}{
		{name: "valid https", param: "token_url", rawURL: "https://auth.example.com/token"},
		{name: "localhost http", param: "token_url", rawURL: "http://localhost:8080/token"},
		{name: "ipv4 loopback http", param: "token_url", rawURL: "http://127.0.0.1:8080/token"},
		{name: "ipv6 loopback http", param: "token_url", rawURL: "http://[::1]:8080/token"},
		{name: "public http rejected", param: "token_url", rawURL: "http://auth.example.com/token", wantError: true},
		{name: "credentials rejected", param: "token_url", rawURL: "https://user:pass@auth.example.com/token", wantError: true},
		{name: "fragment rejected", param: "authorize_url", rawURL: "https://auth.example.com/authorize#frag", wantError: true},
		{name: "query rejected", param: "authorize_url", rawURL: "https://auth.example.com/authorize?prompt=login", wantError: true},
		{name: "relative rejected", param: "token_url", rawURL: "/token", wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDirectOAuthEndpoint(tc.param, tc.rawURL)
			if tc.wantError && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateOAuthIssuerURL(t *testing.T) {
	cases := []struct {
		name      string
		rawURL    string
		wantError bool
	}{
		{name: "valid https", rawURL: "https://auth.example.com/realms/demo"},
		{name: "localhost http", rawURL: "http://localhost:8080"},
		{name: "loopback http", rawURL: "http://127.0.0.1:8080"},
		{name: "public http rejected", rawURL: "http://auth.example.com", wantError: true},
		{name: "userinfo rejected", rawURL: "https://user:pass@auth.example.com", wantError: true},
		{name: "query rejected", rawURL: "https://auth.example.com?tenant=a", wantError: true},
		{name: "fragment rejected", rawURL: "https://auth.example.com#frag", wantError: true},
		{name: "relative rejected", rawURL: "/issuer", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOAuthIssuerURL(tc.rawURL)
			if tc.wantError && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDiscoverOIDCRejectsOversizedBody(t *testing.T) {
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		return testResponse(http.StatusOK, "application/json", strings.Repeat("x", maxOAuthEndpointBodyBytes+1)), nil
	})

	_, err := DiscoverOIDC(context.Background(), client, "https://auth.example.com")
	if err == nil {
		t.Fatal("expected oversized discovery body error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestFetchTokenRejectsOversizedBody(t *testing.T) {
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		return testResponse(http.StatusOK, "application/json", strings.Repeat("x", maxOAuthEndpointBodyBytes+1)), nil
	})

	_, err := FetchToken(context.Background(), client, "https://auth.example.com/token", url.Values{}, nil)
	if err == nil {
		t.Fatal("expected oversized body error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func errorAsToken(err error, target **tokenEndpointError) bool {
	return errors.As(err, target)
}
