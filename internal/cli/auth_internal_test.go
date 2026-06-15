package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/auth"
	authpkg "github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/config"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/spec"
	"github.com/spf13/cobra"
)

func TestRedactDiagnosticAssignmentMultipleSensitiveValues(t *testing.T) {
	input := "access_token=one refresh_token:two client_secret=three password:four"
	got := redactDiagnosticSecretText(input)
	for _, secret := range []string{"one", "two", "three", "four"} {
		if strings.Contains(got, secret) {
			t.Fatalf("redacted text leaked %q: %q", secret, got)
		}
	}
	for _, marker := range []string{"access_token=***", "refresh_token:***", "client_secret=***", "password:***"} {
		if !strings.Contains(got, marker) {
			t.Fatalf("redacted text missing %q: %q", marker, got)
		}
	}
}

func TestCachedOperationSetForAPIUsesEffectiveProfileBase(t *testing.T) {
	cacheDir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "openapi": "3.0.0",
  "info": {"title": "Profile API", "version": "1.0.0"},
  "paths": {
    "/items": {
      "get": {
        "operationId": "profileOperation",
        "responses": {"200": {"description": "ok"}}
      }
    }
  }
}`))
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.Hooks().SpecCachePath = cacheDir
	apiCfg := &config.APIConfig{
		BaseURL:       "https://api.example.com/root",
		OperationBase: "/v1",
		Profiles: map[string]*config.ProfileConfig{
			"default": {
				BaseURL:       srv.URL,
				OperationBase: "/v2",
			},
		},
	}
	if _, err := spec.Discover(context.Background(), spec.DiscoverConfig{
		APIName:       "svc",
		Version:       Version,
		CacheDir:      cacheDir,
		SpecURL:       srv.URL,
		BaseURL:       srv.URL,
		OperationBase: "/v2",
	}, spec.DefaultLoaders()); err != nil {
		t.Fatalf("Discover: %v", err)
	}

	got, ok := c.cachedOperationSetForAPI(context.Background(), "svc", apiCfg, "default")
	if !ok {
		t.Fatal("expected cached profile operation set")
	}
	if len(got.Operations) != 1 || got.Operations[0].ID != "profileOperation" {
		t.Fatalf("cached operation set = %#v, want profile operation", got.Operations)
	}
}

func TestConfiguredCredentialsCountsAuthRef(t *testing.T) {
	apiCfg := &config.APIConfig{
		Profiles: map[string]*config.ProfileConfig{
			"default": {
				Credentials: map[string]*config.CredentialConfig{
					"SharedOAuth": {AuthRef: "shared-oauth"},
					"InlineKey":   {Auth: &config.AuthConfig{Type: "api-key"}},
					"Missing":     {},
				},
			},
		},
	}

	got := configuredCredentials(apiCfg, "default")
	if !got["SharedOAuth"] || !got["InlineKey"] {
		t.Fatalf("configured credentials = %#v, want auth_ref and inline auth counted", got)
	}
	if got["Missing"] {
		t.Fatalf("empty credential counted as configured: %#v", got)
	}
}

func TestSharedAuthCacheKeyIncludesBaseURLForRelativeEndpoints(t *testing.T) {
	ac := &config.AuthConfig{
		Type: "oauth-client-credentials",
		Params: map[string]string{
			"client_id": "myid",
			"token_url": "oauth2/token",
		},
	}
	one := sharedAuthCacheKey("shared", ac, "https://one.example.com/api")
	two := sharedAuthCacheKey("shared", ac, "https://two.example.com/api")
	if one == two {
		t.Fatalf("relative endpoint cache keys should differ by base URL: %q", one)
	}
}

func TestSharedAuthCacheKeyIgnoresBaseURLForAbsoluteEndpoints(t *testing.T) {
	ac := &config.AuthConfig{
		Type: "oauth-client-credentials",
		Params: map[string]string{
			"client_id": "myid",
			"token_url": "https://auth.example.com/oauth2/token",
		},
	}
	one := sharedAuthCacheKey("shared", ac, "https://one.example.com/api")
	two := sharedAuthCacheKey("shared", ac, "https://two.example.com/api")
	if one != two {
		t.Fatalf("absolute endpoint cache keys should ignore API base URL: %q != %q", one, two)
	}
}

func TestInlineAuthCacheKeyIncludesBaseURLForRelativeEndpoints(t *testing.T) {
	ac := &config.AuthConfig{
		Type: "oauth-device-code",
		Params: map[string]string{
			"client_id":                "myid",
			"device_authorization_url": "oauth2/device",
			"token_url":                "oauth2/token",
		},
	}
	one := inlineAuthCacheKey("api:default", ac, "https://one.example.com/api")
	two := inlineAuthCacheKey("api:default", ac, "https://two.example.com/api")
	if one == "" || two == "" || one == two {
		t.Fatalf("inline relative endpoint cache keys = %q and %q, want non-empty and different", one, two)
	}
}

func TestInlineAuthCacheKeyFallsBackForAbsoluteEndpoints(t *testing.T) {
	ac := &config.AuthConfig{
		Type: "oauth-client-credentials",
		Params: map[string]string{
			"client_id": "myid",
			"token_url": "https://auth.example.com/oauth2/token",
		},
	}
	if got := inlineAuthCacheKey("api:default", ac, "https://one.example.com/api"); got != "" {
		t.Fatalf("inline absolute endpoint cache key = %q, want fallback", got)
	}
}

func TestInlineAuthCacheKeyDeduplicatesAbsoluteAuthCodeEndpoints(t *testing.T) {
	// Two APIs pointing at the same dex instance should produce the same cache
	// key so only one browser login is needed.
	ac := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id":     "restish",
			"authorize_url": "https://dex.example.com/auth",
			"token_url":     "https://dex.example.com/token",
		},
	}
	k1 := inlineAuthCacheKey("api-one:default", ac, "https://api-one.example.com")
	k2 := inlineAuthCacheKey("api-two:default", ac, "https://api-two.example.com")
	if k1 == "" || k2 == "" {
		t.Fatalf("expected non-empty cache keys, got %q and %q", k1, k2)
	}
	if k1 != k2 {
		t.Fatalf("expected same cache key for same dex, got %q and %q", k1, k2)
	}

	// Different token_url should give a different key.
	ac2 := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id": "restish",
			"token_url": "https://other-dex.example.com/token",
		},
	}
	k3 := inlineAuthCacheKey("api-one:default", ac2, "https://api-one.example.com")
	if k3 == k1 {
		t.Fatalf("different token_url should produce different cache key, got %q", k3)
	}
}

func TestCachedOAuthTokenEntryMigratesLegacyInlineAuthCodeCacheKey(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "tokens.cbor")
	c := New()
	c.Hooks().TokenCachePath = cacheFile

	ac := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id":     "restish",
			"authorize_url": "https://dex.example.com/auth",
			"token_url":     "https://dex.example.com/token",
		},
	}
	cacheKey := inlineAuthCacheKey("demo:default", ac, "https://api.example.com")
	if cacheKey == "" || cacheKey == "demo:default" {
		t.Fatalf("inline auth code cache key = %q, want hashed oauth key", cacheKey)
	}

	tc := auth.NewTokenCache(cacheFile)
	if err := tc.Set("demo:default", auth.CachedToken{AccessToken: "legacy-token"}); err != nil {
		t.Fatalf("seed legacy token: %v", err)
	}

	got := c.cachedOAuthTokenEntry("oauth-authorization-code", cacheKey, "demo", "default")
	if got == nil || got.AccessToken != "legacy-token" {
		t.Fatalf("migrated token = %+v, want legacy-token", got)
	}
	migrated, err := tc.Get(cacheKey)
	if err != nil {
		t.Fatalf("read migrated token: %v", err)
	}
	if migrated == nil || migrated.AccessToken != "legacy-token" {
		t.Fatalf("migrated cache entry = %+v, want legacy-token", migrated)
	}
	legacy, err := tc.Get("demo:default")
	if err != nil {
		t.Fatalf("read legacy token: %v", err)
	}
	if legacy != nil {
		t.Fatalf("legacy cache entry still present: %+v", legacy)
	}
}

func TestInlineAuthCacheKeyDeduplicatesIssuerAuthCodeEndpoints(t *testing.T) {
	ac := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id":  "restish",
			"issuer_url": "https://dex.example.com",
		},
	}
	k1 := inlineAuthCacheKey("api-one:default", ac, "https://api-one.example.com")
	k2 := inlineAuthCacheKey("api-two:default", ac, "https://api-two.example.com")
	if k1 == "" || k2 == "" {
		t.Fatalf("expected non-empty cache keys, got %q and %q", k1, k2)
	}
	if k1 != k2 {
		t.Fatalf("expected same cache key for same issuer, got %q and %q", k1, k2)
	}

	ac2 := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id":  "restish",
			"issuer_url": "https://other-dex.example.com",
		},
	}
	k3 := inlineAuthCacheKey("api-one:default", ac2, "https://api-one.example.com")
	if k3 == k1 {
		t.Fatalf("different issuer_url should produce different cache key, got %q", k3)
	}

	ac3 := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id":  "restish",
			"issuer_url": "https://dex.example.com",
			"scopes":     "admin",
		},
	}
	k4 := inlineAuthCacheKey("api-one:default", ac3, "https://api-one.example.com")
	if k4 == k1 {
		t.Fatalf("different issuer_url scopes should produce different cache key, got %q", k4)
	}
}

func TestInlineAuthCacheKeySeparatesTokenShapingAuthCodeParams(t *testing.T) {
	read := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id": "restish",
			"token_url": "https://dex.example.com/token",
			"scopes":    "read",
		},
	}
	write := &config.AuthConfig{
		Type: "oauth-authorization-code",
		Params: map[string]string{
			"client_id": "restish",
			"token_url": "https://dex.example.com/token",
			"scopes":    "write",
		},
	}
	k1 := inlineAuthCacheKey("api-one:default", read, "https://api-one.example.com")
	k2 := inlineAuthCacheKey("api-two:default", write, "https://api-two.example.com")
	if k1 == "" || k2 == "" {
		t.Fatalf("expected non-empty cache keys, got %q and %q", k1, k2)
	}
	if k1 == k2 {
		t.Fatalf("different scopes should produce different cache keys, got %q", k1)
	}
}

func TestSharedDiscoveryTransportDoesNotShareDifferentTLSSignerParams(t *testing.T) {
	signerPath := filepath.Join(t.TempDir(), "restish-test-tls-signer")
	if err := os.WriteFile(signerPath, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write signer: %v", err)
	}
	c := New()
	c.cfg = &config.Config{APIs: map[string]*config.APIConfig{
		"one": {
			BaseURL: "https://one.example.com",
			Profiles: map[string]*config.ProfileConfig{"default": {
				TLSSigner:       signerPath,
				TLSSignerParams: map[string]string{"slot": "1"},
			}},
		},
		"two": {
			BaseURL: "https://two.example.com",
			Profiles: map[string]*config.ProfileConfig{"default": {
				TLSSigner:       signerPath,
				TLSSignerParams: map[string]string{"slot": "2"},
			}},
		},
	}}

	tr, closer, err := c.sharedDiscoveryTransport(nil, []string{"one", "two"})
	if err != nil {
		t.Fatalf("shared discovery transport: %v", err)
	}
	if tr != nil || closer != nil {
		t.Fatalf("shared discovery transport = %T, %T; want no shared transport", tr, closer)
	}
}

func TestSharedDiscoveryTransportWarnsWhenInsecure(t *testing.T) {
	signerPath := filepath.Join(t.TempDir(), "restish-test-tls-signer")
	if err := os.WriteFile(signerPath, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write signer: %v", err)
	}
	var stderr bytes.Buffer
	c := New()
	c.Stderr = &stderr
	c.cfg = &config.Config{APIs: map[string]*config.APIConfig{
		"one": {
			BaseURL: "https://one.example.com",
			Profiles: map[string]*config.ProfileConfig{"default": {
				TLSSigner:       signerPath,
				TLSSignerParams: map[string]string{"slot": "1"},
			}},
		},
		"two": {
			BaseURL: "https://two.example.com",
			Profiles: map[string]*config.ProfileConfig{"default": {
				TLSSigner:       signerPath,
				TLSSignerParams: map[string]string{"slot": "1"},
			}},
		},
	}}
	cmd := &cobra.Command{}
	cmd.SetContext(withGlobalFlags(context.Background(), GlobalFlags{Insecure: true, Retry: -1, MaxPages: 25}))

	if _, _, err := c.sharedDiscoveryTransport(cmd, []string{"one", "two"}); err != nil {
		t.Fatalf("shared discovery transport: %v", err)
	}
	if got := stderr.String(); !strings.Contains(got, "TLS certificate verification is disabled (--rsh-insecure)") {
		t.Fatalf("stderr = %q, want insecure warning", got)
	}
}

func TestAuthHandlerForOAuthUsesThemeCallbackColors(t *testing.T) {
	if err := output.SetTheme(output.ThemeEntries{
		"status_2xx":   "bold #00ff00",
		"status_error": "italic #ff0000",
	}); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	t.Cleanup(func() {
		if err := output.SetTheme(nil); err != nil {
			t.Fatalf("reset theme: %v", err)
		}
	})

	c := New()
	handler, err := c.authHandlerFor(&config.AuthConfig{Type: "oauth-authorization-code"}, authHandlerOptions{})
	if err != nil {
		t.Fatalf("authHandlerFor: %v", err)
	}
	oauthHandler, ok := handler.(*authpkg.AuthorizationCode)
	if !ok {
		t.Fatalf("handler = %T, want *authpkg.AuthorizationCode", handler)
	}
	if oauthHandler.CallbackSuccessColor != "#00ff00" {
		t.Fatalf("success color = %q, want #00ff00", oauthHandler.CallbackSuccessColor)
	}
	if oauthHandler.CallbackFailureColor != "#ff0000" {
		t.Fatalf("failure color = %q, want #ff0000", oauthHandler.CallbackFailureColor)
	}
}

func TestOperationSetForAPIHintsAPISync_WhenCacheEmpty(t *testing.T) {
	var stderr bytes.Buffer
	c := New()
	c.Stderr = &stderr
	c.Hooks().SpecCachePath = t.TempDir() // empty cache dir

	apiCfg := &config.APIConfig{BaseURL: "https://api.example.com"}
	set, ok, err := c.operationSetForAPI(context.Background(), "example", apiCfg, "default", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok || len(set.Operations) > 0 {
		t.Fatal("expected empty operation set from cold cache")
	}
	if !strings.Contains(stderr.String(), "api sync example") {
		t.Fatalf("expected api sync hint on stderr, got: %q", stderr.String())
	}
}

func TestOperationSetForAPINoHintWhenForceRefresh(t *testing.T) {
	// forceRefresh=true means an explicit sync is already in progress; no hint.
	var stderr bytes.Buffer
	c := New()
	c.Stderr = &stderr
	c.Hooks().SpecCachePath = t.TempDir()

	apiCfg := &config.APIConfig{BaseURL: "https://203.0.113.1"}
	_, _, _ = c.operationSetForAPI(context.Background(), "example", apiCfg, "default", true)
	if strings.Contains(stderr.String(), "api sync") {
		t.Fatalf("expected no api sync hint during forceRefresh, got: %q", stderr.String())
	}
}

func TestOperationSetForAPINoHintWhenNoSpecSource(t *testing.T) {
	// An API with no BaseURL, no SpecURL, no SpecFiles configured has nothing to sync.
	var stderr bytes.Buffer
	c := New()
	c.Stderr = &stderr
	c.Hooks().SpecCachePath = t.TempDir()

	apiCfg := &config.APIConfig{}
	_, _, _ = c.operationSetForAPI(context.Background(), "bare", apiCfg, "default", false)
	if strings.Contains(stderr.String(), "api sync") {
		t.Fatalf("expected no api sync hint for API with no spec source, got: %q", stderr.String())
	}
}
