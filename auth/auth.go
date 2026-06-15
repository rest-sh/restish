package auth

import (
	"context"
	"io"
	"maps"
	"net/http"
)

// Param describes a configuration parameter required by an auth handler.
type Param struct {
	Name        string
	Description string
	Required    bool
	Secret      bool // true -> don't echo when prompting
}

// TokenStore persists OAuth-style bearer tokens keyed by API/profile.
type TokenStore interface {
	Get(key string) (*CachedToken, error)
	Set(key string, token CachedToken) error
	Delete(key string) error
	DeletePrefix(prefix string) error
}

// Prompter reads interactive values from the user.
type Prompter interface {
	Prompt(prompt string) (string, error)
	PromptSecret(prompt string) (string, error)
}

// Logger is the minimal logging surface auth handlers use for diagnostics.
type Logger interface {
	Printf(format string, v ...any)
}

// AuthContext carries the request-scoped auth environment into a handler.
type AuthContext struct {
	APIName     string
	ProfileName string
	BaseURL     string
	CacheKey    string
	Params      map[string]string // user-supplied only
	TokenStore  TokenStore
	Prompter    Prompter
	Stderr      io.Writer
	HTTPClient  *http.Client
	Logger      Logger
	Force       bool // bypass cached access tokens for a single retry
}

// Handler is implemented by each auth mechanism.
type Handler interface {
	// Parameters returns the list of configuration parameters this handler needs.
	Parameters() []Param
	// Authenticate mutates req to add authentication credentials.
	Authenticate(ctx context.Context, req *http.Request, ac AuthContext) error
}

// ForceCapable marks handlers that can meaningfully bypass cached credentials
// after a 401 and retry once with fresh auth state.
type ForceCapable interface {
	SupportsForce()
}

func bearerAuth(req *http.Request, token string) {
	if getHeaderCaseInsensitive(req.Header, "Authorization") != "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}

// authParams returns a copy of the user-supplied auth params with a synthetic
// "_cache_key" entry added so token-cache lookups have a stable identity even
// when callers pass an empty CacheKey.
func authParams(ac AuthContext) map[string]string {
	params := make(map[string]string, len(ac.Params)+1)
	maps.Copy(params, ac.Params)
	switch {
	case ac.CacheKey != "":
		params["_cache_key"] = ac.CacheKey
	case ac.APIName != "" || ac.ProfileName != "":
		params["_cache_key"] = ac.APIName + ":" + ac.ProfileName
	}
	if ac.BaseURL != "" {
		params["_base_url"] = ac.BaseURL
	}
	return params
}
