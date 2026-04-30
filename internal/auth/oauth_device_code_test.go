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
	h := &DeviceCode{
		Cache: cache,
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
