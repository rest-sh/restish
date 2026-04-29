package cli

import (
	"context"
	"net/http"
	"strings"
	"testing"

	authpkg "github.com/rest-sh/restish/v2/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/spec"
)

type forceRecordingAuth struct {
	forces []bool
}

func (h *forceRecordingAuth) Parameters() []authpkg.Param { return nil }

func (h *forceRecordingAuth) Authenticate(_ context.Context, req *http.Request, ac authpkg.AuthContext) error {
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
				Auth:      &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": "header", "name": "X-User-Key", "value": "secret"}},
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
