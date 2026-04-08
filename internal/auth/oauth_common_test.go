package auth

import "testing"

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
