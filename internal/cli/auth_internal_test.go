package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/spec"
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
	oauthHandler, ok := handler.(*auth.AuthorizationCode)
	if !ok {
		t.Fatalf("handler = %T, want *auth.AuthorizationCode", handler)
	}
	if oauthHandler.CallbackSuccessColor != "#00ff00" {
		t.Fatalf("success color = %q, want #00ff00", oauthHandler.CallbackSuccessColor)
	}
	if oauthHandler.CallbackFailureColor != "#ff0000" {
		t.Fatalf("failure color = %q, want #ff0000", oauthHandler.CallbackFailureColor)
	}
}
