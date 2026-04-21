package auth

import (
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

func errorAsToken(err error, target **tokenEndpointError) bool {
	te, ok := err.(*tokenEndpointError)
	if ok {
		*target = te
	}
	return ok
}
