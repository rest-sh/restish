package auth

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/auth"
)

// Supabase GoTrue rejects a dead refresh token with 400
// {"error_code":"refresh_token_not_found"} instead of {"error":"invalid_grant"}.
// That must clear the cached token and fall through to interactive auth, not
// fail every request until a manual logout.
func TestDeviceCodeProviderRefreshRejectionFallsThrough(t *testing.T) {
	cache := auth.NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor"))
	if err := cache.Set("svc:default", auth.CachedToken{
		AccessToken:  "expired-token",
		RefreshToken: "rejected-refresh",
		Expiry:       time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	var deviceStarted bool
	var stderr strings.Builder
	h := &DeviceCode{
		Cache:  cache,
		Stderr: &stderr,
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			switch r.URL.String() {
			case "https://auth.example.com/token":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm: %v", err)
				}
				if r.FormValue("grant_type") == "refresh_token" {
					return testResponse(400, "application/json", `{"code":400,"error_code":"refresh_token_not_found","msg":"Invalid Refresh Token: Refresh Token Not Found"}`), nil
				}
				return testResponse(200, "application/json", `{"access_token":"device-token","token_type":"bearer","expires_in":3600}`), nil
			case "https://auth.example.com/device":
				deviceStarted = true
				return testResponse(200, "application/json", `{
					"device_code":"device-123",
					"user_code":"ABCD-EFGH",
					"verification_uri":"https://verify.example.com",
					"interval":1,
					"expires_in":60
				}`), nil
			default:
				t.Fatalf("unexpected URL %q", r.URL.String())
				return nil, nil
			}
		}),
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"_cache_key":               "svc:default",
		"client_id":                "id1",
		"device_authorization_url": "https://auth.example.com/device",
		"token_url":                "https://auth.example.com/token",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if !deviceStarted {
		t.Fatal("expected provider refresh rejection to fall through to device flow")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer device-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if !strings.Contains(stderr.String(), "cleared cached token") {
		t.Fatalf("expected cache clear warning, got %q", stderr.String())
	}
}
