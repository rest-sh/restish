package auth

import (
	"errors"
	"testing"
)

func TestIsRefreshTokenRejected(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"rfc invalid_grant", parseTokenEndpointError(400, []byte(`{"error":"invalid_grant"}`)), true},
		{"gotrue refresh_token_not_found", parseTokenEndpointError(400, []byte(`{"code":400,"error_code":"refresh_token_not_found","msg":"Invalid Refresh Token: Refresh Token Not Found"}`)), true},
		{"gotrue refresh_token_already_used", parseTokenEndpointError(400, []byte(`{"error_code":"refresh_token_already_used"}`)), true},
		{"gotrue session_expired", parseTokenEndpointError(403, []byte(`{"error_code":"session_expired"}`)), true},
		{"rfc invalid_client", parseTokenEndpointError(401, []byte(`{"error":"invalid_client"}`)), false},
		{"gotrue unrelated code", parseTokenEndpointError(400, []byte(`{"error_code":"validation_failed"}`)), false},
		{"server error", parseTokenEndpointError(502, []byte(`upstream down`)), false},
		{"network", errors.New("dial failed"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRefreshTokenRejected(tc.err); got != tc.want {
				t.Fatalf("isRefreshTokenRejected(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
