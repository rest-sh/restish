package auth

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeviceCode_PollsUntilAuthorized(t *testing.T) {
	var polls int
	var gotStart url.Values
	h := &DeviceCode{
		Cache: NewTokenCache(filepath.Join(t.TempDir(), "tokens.json")),
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			switch r.URL.String() {
			case "https://auth.example.com/device":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm: %v", err)
				}
				gotStart = r.Form
				return testResponse(200, "application/json", `{
					"device_code":"device-123",
					"user_code":"ABCD-EFGH",
					"verification_uri":"https://verify.example.com",
					"verification_uri_complete":"https://verify.example.com/complete",
					"interval":1,
					"expires_in":60
				}`), nil
			case "https://auth.example.com/token":
				polls++
				if polls == 1 {
					return testResponse(400, "application/json", `{"error":"authorization_pending"}`), nil
				}
				if err := r.ParseForm(); err != nil {
					t.Fatalf("ParseForm: %v", err)
				}
				if got := r.FormValue("device_code"); got != "device-123" {
					t.Fatalf("device_code = %q", got)
				}
				for _, key := range []string{"redirect_scheme", "redirect_port", "redirect_path", "redirect_cert", "redirect_key"} {
					if got := r.Form.Get(key); got != "" {
						t.Fatalf("%s leaked into device token request: %#v", key, r.Form)
					}
				}
				return testResponse(200, "application/json", `{"access_token":"device-token","token_type":"bearer","expires_in":3600}`), nil
			default:
				t.Fatalf("unexpected URL %q", r.URL.String())
				return nil, nil
			}
		}),
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":                "id1",
		"device_authorization_url": "https://auth.example.com/device",
		"token_url":                "https://auth.example.com/token",
		"audience":                 "https://api.example.com/",
		"_base_url":                "https://api.example.com/v1",
		"cache_key":                "local-cache-key",
		"redirect_scheme":          "https",
		"redirect_port":            "8484",
		"redirect_path":            "/callback",
		"redirect_cert":            "./localhost.pem",
		"redirect_key":             "./localhost.key",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer device-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if gotStart.Get("audience") != "https://api.example.com/" {
		t.Fatalf("expected passthrough audience in device auth request, got %#v", gotStart)
	}
	if gotStart.Get("cache_key") != "" {
		t.Fatalf("cache_key leaked into device auth request: %#v", gotStart)
	}
	if gotStart.Get("_base_url") != "" {
		t.Fatalf("_base_url leaked into device auth request: %#v", gotStart)
	}
	for _, key := range []string{"redirect_scheme", "redirect_port", "redirect_path", "redirect_cert", "redirect_key"} {
		if got := gotStart.Get(key); got != "" {
			t.Fatalf("%s leaked into device auth request: %#v", key, gotStart)
		}
	}
}

func TestDeviceCode_OIDCDiscovery(t *testing.T) {
	h := &DeviceCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			switch r.URL.String() {
			case "https://issuer.example.com/.well-known/openid-configuration":
				return testResponse(200, "application/json", `{
					"device_authorization_endpoint":"https://issuer.example.com/device",
					"token_endpoint":"https://issuer.example.com/token"
				}`), nil
			case "https://issuer.example.com/device":
				return testResponse(200, "application/json", `{
					"device_code":"device-123",
					"user_code":"ABCD-EFGH",
					"verification_uri":"https://verify.example.com",
					"interval":1,
					"expires_in":10
				}`), nil
			case "https://issuer.example.com/token":
				return testResponse(200, "application/json", `{"access_token":"device-token","token_type":"bearer","expires_in":3600}`), nil
			default:
				t.Fatalf("unexpected URL %q", r.URL.String())
				return nil, nil
			}
		}),
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"client_id":  "id1",
		"issuer_url": "https://issuer.example.com",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer device-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestDeviceCodeResolvesRelativeEndpoints(t *testing.T) {
	h := &DeviceCode{}
	deviceURL, tokenURL, err := h.resolveEndpoints(context.Background(), map[string]string{
		"device_authorization_url": "oauth2/device",
		"token_url":                "/oauth2/token",
		"_base_url":                "https://api.example.com/v1",
	})
	if err != nil {
		t.Fatalf("resolveEndpoints: %v", err)
	}
	if deviceURL != "https://api.example.com/v1/oauth2/device" {
		t.Fatalf("deviceURL = %q", deviceURL)
	}
	if tokenURL != "https://api.example.com/oauth2/token" {
		t.Fatalf("tokenURL = %q", tokenURL)
	}
}

func TestDeviceCode_RefreshesCachedToken(t *testing.T) {
	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor"))
	if err := cache.Set("svc:default", CachedToken{
		AccessToken:  "expired-token",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	var refreshed bool
	h := &DeviceCode{
		Cache: cache,
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://auth.example.com/token" {
				t.Fatalf("unexpected URL %q", r.URL.String())
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			if got := r.FormValue("grant_type"); got != "refresh_token" {
				t.Fatalf("grant_type = %q", got)
			}
			if got := r.FormValue("refresh_token"); got != "refresh-token" {
				t.Fatalf("refresh_token = %q", got)
			}
			refreshed = true
			return testResponse(200, "application/json", `{"access_token":"fresh-token","token_type":"bearer","expires_in":3600}`), nil
		}),
	}

	req, _ := http.NewRequest("GET", "https://api.example.com", nil)
	params := map[string]string{
		"_cache_key": "svc:default",
		"client_id":  "id1",
		"token_url":  "https://auth.example.com/token",
	}
	if err := h.OnRequest(req, params); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if !refreshed {
		t.Fatal("expected refresh request")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer fresh-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestDeviceCode_InvalidGrantFallsThroughToDeviceFlow(t *testing.T) {
	cache := NewTokenCache(filepath.Join(t.TempDir(), "tokens.cbor"))
	if err := cache.Set("svc:default", CachedToken{
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
					return testResponse(400, "application/json", `{"error":"invalid_grant"}`), nil
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
		t.Fatal("expected invalid_grant to fall through to device flow")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer device-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if !strings.Contains(stderr.String(), "cleared cached token") {
		t.Fatalf("expected invalid_grant cache clear warning, got %q", stderr.String())
	}
}

func TestDeviceCodeRequestRejectsOversizedBody(t *testing.T) {
	h := &DeviceCode{
		HTTPClient: testHTTPClient(func(r *http.Request) (*http.Response, error) {
			return testResponse(http.StatusOK, "application/json", strings.Repeat("x", maxOAuthEndpointBodyBytes+1)), nil
		}),
	}

	_, err := h.requestDeviceAuthorization(context.Background(), map[string]string{"client_id": "id1"}, "https://auth.example.com/device")
	if err == nil {
		t.Fatal("expected oversized body error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestDeviceCodeSlowDownIntervalIsCapped(t *testing.T) {
	if got := capDevicePollInterval(35 * time.Second); got != 30*time.Second {
		t.Fatalf("capDevicePollInterval(35s) = %v, want 30s", got)
	}
	if got := capDevicePollInterval(25 * time.Second); got != 25*time.Second {
		t.Fatalf("capDevicePollInterval(25s) = %v, want 25s", got)
	}
}
