package auth

import (
	"context"
	"io"
	"log"
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

// ForceCapable is implemented by handlers that can meaningfully bypass cached
// credentials after a 401 and retry once with fresh auth state.
type ForceCapable interface {
	SupportsForce() bool
}

func authCacheKey(ac AuthContext) string {
	if ac.APIName == "" && ac.ProfileName == "" {
		return ""
	}
	return ac.APIName + ":" + ac.ProfileName
}

func authLogger(ac AuthContext) Logger {
	if ac.Logger != nil {
		return ac.Logger
	}
	return log.New(io.Discard, "", 0)
}

func cloneAuthParams(params map[string]string) map[string]string {
	cloned := make(map[string]string, len(params)+1)
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}
