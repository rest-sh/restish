package cli

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

type forceRecordingAuth struct {
	forces []bool
}

func (h *forceRecordingAuth) Parameters() []auth.Param { return nil }

func (h *forceRecordingAuth) Authenticate(_ context.Context, req *http.Request, ac auth.AuthContext) error {
	h.forces = append(h.forces, ac.Force)
	req.Header.Set("Authorization", "Bearer token")
	return nil
}

func (h *forceRecordingAuth) SupportsForce() {}

func TestPlanOperationAuthRejectsMissingRequirementValues(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"UserOAuth": {
				Auth:      &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-User-Key", "value": "secret", "scopes": "items:read"}},
				Satisfies: []string{"items:write"},
			},
		},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{{
		{ID: "UserOAuth", Needs: []string{"items:read"}},
	}}}

	_, _, err := c.planOperationAuth("svc", "default", prof, policy)
	if err == nil {
		t.Fatal("expected missing requirement value error")
	}
	if !strings.Contains(err.Error(), "do not satisfy") || !strings.Contains(err.Error(), "items:read") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanOperationAuthDerivesSatisfiesFromAuthProfileScopes(t *testing.T) {
	c := &CLI{cfg: &config.Config{
		AuthProfiles: map[string]*config.AuthConfig{
			"shared-oauth": {
				Type:   "api-key",
				Params: map[string]string{"in": "header", "name": "Authorization", "value": "Bearer token", "scopes": "items:read items:write"},
			},
		},
	}}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"UserOAuth": {AuthRef: "shared-oauth"},
		},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{{
		{ID: "UserOAuth", Needs: []string{"items:read"}},
	}}}

	selected, handled, err := c.planOperationAuth("svc", "default", prof, policy)
	if err != nil {
		t.Fatalf("planOperationAuth: %v", err)
	}
	if !handled || len(selected) != 1 || selected[0].resolved.Ref != "shared-oauth" {
		t.Fatalf("selected = %#v handled=%v, want shared auth profile", selected, handled)
	}
}

func TestPlanOperationAuthRejectsAmbiguousProfileFallback(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": "u", "password": "p"}},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{
		{{ID: "UserOAuth"}},
		{{ID: "PartnerKey"}},
	}}

	_, _, err := c.planOperationAuth("svc", "default", prof, policy)
	if err == nil {
		t.Fatal("expected missing credential binding error")
	}
	if !strings.Contains(err.Error(), "missing credential bindings") ||
		!strings.Contains(err.Error(), "UserOAuth") ||
		!strings.Contains(err.Error(), "PartnerKey") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanOperationAuthMissingCredentialSuggestsExplicitConfiguredOverride(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"BearerAuth": {
				Auth: &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "secret"}},
			},
		},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{{
		{ID: "PartnerKey"},
	}}}

	_, _, err := c.planOperationAuth("svc", "default", prof, policy)
	if err == nil {
		t.Fatal("expected missing credential binding error")
	}
	if !strings.Contains(err.Error(), `configured credential "BearerAuth" is not declared for this operation`) ||
		!strings.Contains(err.Error(), "--rsh-auth BearerAuth") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanOperationAuthAllowsSingleRequirementProfileFallback(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": "u", "password": "p"}},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{{
		{ID: "BasicAuth"},
	}}}

	selected, handled, err := c.planOperationAuth("svc", "default", prof, policy)
	if err != nil {
		t.Fatalf("planOperationAuth: %v", err)
	}
	if !handled || len(selected) != 1 || selected[0].resolved.Config != prof.Auth {
		t.Fatalf("selected = %#v handled=%v, want profile fallback", selected, handled)
	}
}

func TestPlanOperationAuthPrefersCredentialsBeforeAnonymous(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"PartnerKey": {
				Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Partner-Key", "value": "secret"}},
			},
		},
	}
	policy := &operationAuthPolicy{
		OptionalAuth: true,
		CredentialAlternatives: []spec.CredentialAlternative{{
			{ID: "PartnerKey"},
		}},
	}

	selected, handled, err := c.planOperationAuth("svc", "default", prof, policy)
	if err != nil {
		t.Fatalf("planOperationAuth: %v", err)
	}
	if !handled || len(selected) != 1 || selected[0].requirement.ID != "PartnerKey" {
		t.Fatalf("selected = %#v handled=%v, want PartnerKey", selected, handled)
	}
}

func TestPlanOperationAuthSkipsAlternativeWithMissingEnvParam(t *testing.T) {
	t.Setenv("READY_KEY", "ready")
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"MissingKey": {
				Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Missing", "value": "env:MISSING_KEY"}},
			},
			"ReadyKey": {
				Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Ready", "value": "env:READY_KEY"}},
			},
		},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{
		{{ID: "MissingKey"}},
		{{ID: "ReadyKey"}},
	}}

	selected, handled, err := c.planOperationAuth("svc", "default", prof, policy)
	if err != nil {
		t.Fatalf("planOperationAuth: %v", err)
	}
	if !handled || len(selected) != 1 || selected[0].requirement.ID != "ReadyKey" {
		t.Fatalf("selected = %#v handled=%v, want ReadyKey", selected, handled)
	}
}

func TestPlanOperationAuthReportsUnresolvedEnvWhenNoAlternativeReady(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"MissingKey": {
				Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Missing", "value": "env:MISSING_KEY"}},
			},
		},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{{{ID: "MissingKey"}}}}

	_, _, err := c.planOperationAuth("svc", "default", prof, policy)
	if err == nil || !strings.Contains(err.Error(), "unresolved auth params") || !strings.Contains(err.Error(), "env:MISSING_KEY") {
		t.Fatalf("error = %v, want unresolved env diagnostic", err)
	}
}

func TestPlanOperationAuthUsesAnonymousWhenOptionalCredentialMissing(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{}
	policy := &operationAuthPolicy{
		OptionalAuth: true,
		CredentialAlternatives: []spec.CredentialAlternative{{
			{ID: "PartnerKey"},
		}},
	}

	selected, handled, err := c.planOperationAuth("svc", "default", prof, policy)
	if err != nil {
		t.Fatalf("planOperationAuth: %v", err)
	}
	if !handled || len(selected) != 0 {
		t.Fatalf("selected = %#v handled=%v, want anonymous", selected, handled)
	}
}

func TestPlanOperationAuthOverrideValidation(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"PartnerKey": {
				Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Partner-Key", "value": "secret"}},
			},
		},
	}
	basePolicy := []spec.CredentialAlternative{
		{{ID: "UserOAuth", Needs: []string{"items:read"}}},
		{{ID: "PartnerKey"}},
	}

	selected, handled, err := c.planOperationAuth("svc", "default", prof, &operationAuthPolicy{
		CredentialAlternatives: basePolicy,
		Override:               "PartnerKey",
	})
	if err != nil {
		t.Fatalf("valid override: %v", err)
	}
	if !handled || len(selected) != 1 || selected[0].requirement.ID != "PartnerKey" {
		t.Fatalf("selected = %#v handled=%v, want PartnerKey", selected, handled)
	}

	_, _, err = c.planOperationAuth("svc", "default", prof, &operationAuthPolicy{
		CredentialAlternatives: basePolicy,
		Override:               "UserOAuth+PartnerKey",
	})
	if err == nil || !strings.Contains(err.Error(), "requires missing credential bindings") {
		t.Fatalf("expected missing binding error, got %v", err)
	}

	_, _, err = c.planOperationAuth("svc", "default", prof, &operationAuthPolicy{
		CredentialAlternatives: basePolicy,
		Override:               "UserOAuth",
	})
	if err == nil || !strings.Contains(err.Error(), "requires missing credential bindings") {
		t.Fatalf("expected missing binding error, got %v", err)
	}

	selected, handled, err = c.planOperationAuth("svc", "default", prof, &operationAuthPolicy{
		OptionalAuth:           true,
		CredentialAlternatives: basePolicy,
		Override:               "anonymous",
	})
	if err != nil {
		t.Fatalf("anonymous override: %v", err)
	}
	if !handled || len(selected) != 0 {
		t.Fatalf("selected = %#v handled=%v, want anonymous", selected, handled)
	}
}

func TestPlanOperationAuthOverrideAllowsConfiguredCredentialOutsideSpec(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"BearerAuth": {
				Auth: &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "manual"}},
			},
		},
	}
	policy := &operationAuthPolicy{
		CredentialAlternatives: []spec.CredentialAlternative{{{ID: "LegacyKey"}}},
		Override:               "BearerAuth",
	}

	selected, handled, err := c.planOperationAuth("svc", "default", prof, policy)
	if err != nil {
		t.Fatalf("planOperationAuth: %v", err)
	}
	if !handled || len(selected) != 1 || selected[0].requirement.ID != "BearerAuth" {
		t.Fatalf("selected = %#v handled=%v, want BearerAuth", selected, handled)
	}
}

func TestPlanOperationAuthRejectsCredentialMutationConflict(t *testing.T) {
	c := &CLI{}
	prof := &config.ProfileConfig{
		Credentials: map[string]*config.CredentialConfig{
			"A": {Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-API-Key", "value": "a"}}},
			"B": {Auth: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "x-api-key", "value": "b"}}},
		},
	}
	policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{{
		{ID: "A"},
		{ID: "B"},
	}}}

	_, _, err := c.planOperationAuth("svc", "default", prof, policy)
	if err == nil {
		t.Fatal("expected credential mutation conflict")
	}
	if !strings.Contains(err.Error(), "both write header:x-api-key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanOperationAuthRejectsBearerAuthorizationConflicts(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth *config.AuthConfig
	}{
		{
			name: "http-basic",
			auth: &config.AuthConfig{Type: "http-basic", Params: map[string]string{
				"username": "u",
				"password": "p",
			}},
		},
		{
			name: "oauth2",
			auth: &config.AuthConfig{Type: "oauth-client-credentials", Params: map[string]string{
				"client_id":     "id",
				"client_secret": "secret",
				"token_url":     "https://auth.example.com/token",
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &CLI{}
			prof := &config.ProfileConfig{
				Credentials: map[string]*config.CredentialConfig{
					"Bearer": {Auth: &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": "abc"}}},
					"Other":  {Auth: tc.auth},
				},
			}
			policy := &operationAuthPolicy{CredentialAlternatives: []spec.CredentialAlternative{{
				{ID: "Bearer"},
				{ID: "Other"},
			}}}

			_, _, err := c.planOperationAuth("svc", "default", prof, policy)
			if err == nil {
				t.Fatal("expected credential mutation conflict")
			}
			if !strings.Contains(err.Error(), "both write header:authorization") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestOperationAuthCallbacksForceCapableUnauthorizedRetry(t *testing.T) {
	c := New()
	handler := &forceRecordingAuth{}
	c.AddAuthHandler("force-test", handler)
	selected := []selectedOperationAuth{{
		requirement: spec.CredentialRequirement{ID: "UserOAuth"},
		resolved: resolvedAuthConfig{
			Config: &config.AuthConfig{Type: "force-test"},
		},
	}}

	callbacks, err := c.operationAuthCallbacks("svc", "default", selected, authHandlerOptions{})
	if err != nil {
		t.Fatalf("operationAuthCallbacks: %v", err)
	}
	if callbacks.OnRequest == nil || callbacks.OnUnauthorized == nil {
		t.Fatal("expected request and unauthorized callbacks")
	}

	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	if err := callbacks.OnRequest(req); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if err := callbacks.OnUnauthorized(req); err != nil {
		t.Fatalf("OnUnauthorized: %v", err)
	}
	if len(handler.forces) != 2 || handler.forces[0] || !handler.forces[1] {
		t.Fatalf("forces = %#v, want [false true]", handler.forces)
	}
}

func TestOperationAuthCallbacksRunHookOnceForMultipleCredentials(t *testing.T) {
	c := New()
	var hookCalls atomic.Int32
	c.Hooks().AuthHookFunc = func(apiName, profileName string, rawParams map[string]string, secretKeys map[string]bool, req *http.Request) error {
		hookCalls.Add(1)
		req.Header.Set("X-Hook", "called")
		return nil
	}
	selected := []selectedOperationAuth{
		{
			requirement: spec.CredentialRequirement{ID: "FirstKey"},
			resolved: resolvedAuthConfig{
				Config: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-First-Key", "value": "one"}},
			},
		},
		{
			requirement: spec.CredentialRequirement{ID: "SecondKey"},
			resolved: resolvedAuthConfig{
				Config: &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-Second-Key", "value": "two"}},
			},
		},
	}

	callbacks, err := c.operationAuthCallbacks("svc", "default", selected, authHandlerOptions{})
	if err != nil {
		t.Fatalf("operationAuthCallbacks: %v", err)
	}
	req, _ := http.NewRequest("GET", "https://api.example.com/items", nil)
	if err := callbacks.OnRequest(req); err != nil {
		t.Fatalf("OnRequest: %v", err)
	}
	if got := hookCalls.Load(); got != 1 {
		t.Fatalf("hook calls = %d, want 1", got)
	}
	if req.Header.Get("X-First-Key") != "one" || req.Header.Get("X-Second-Key") != "two" || req.Header.Get("X-Hook") != "called" {
		t.Fatalf("headers after operation auth = %#v", req.Header)
	}
}
