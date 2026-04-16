package cli

import (
	"fmt"
	"net/http"

	"github.com/danielgtaylor/restish/v2/internal/auth"
	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/spf13/cobra"
)

// addAuthHeaderCommand registers the "auth-header" command on root.
func (c *CLI) addAuthHeaderCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "auth-header <api>",
		Short: "Print the Authorization header value for a registered API",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runAuthHeader,
	})
}

// runAuthHeader resolves auth for the named API and prints the Authorization
// header value.
func (c *CLI) runAuthHeader(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q", apiName)
	}
	api := c.cfg.APIs[apiName]

	profileName := c.profileFromCmd(cmd)

	if api.Profiles == nil || api.Profiles[profileName] == nil {
		return fmt.Errorf("API %q has no profile %q", apiName, profileName)
	}
	prof := api.Profiles[profileName]
	if prof.Auth == nil {
		return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
	}

	handler, err := c.authHandlerFor(prof.Auth)
	if err != nil {
		return err
	}

	params := c.buildAuthParams(apiName, profileName, prof.Auth.Params)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := handler.OnRequest(req, params); err != nil {
		return fmt.Errorf("building auth header: %w", err)
	}

	authVal := req.Header.Get("Authorization")
	if authVal == "" {
		return fmt.Errorf("auth handler did not set an Authorization header")
	}
	fmt.Fprintln(c.Stdout, authVal)
	return nil
}

// authHandlerFor returns the Handler for the given AuthConfig.
// Custom handlers registered via CLI.AddAuthHandler take precedence over
// built-in handlers.
func (c *CLI) authHandlerFor(ac *config.AuthConfig) (auth.Handler, error) {
	if c.customAuthHandlers != nil {
		if h, ok := c.customAuthHandlers[ac.Type]; ok {
			return h, nil
		}
	}
	switch ac.Type {
	case "http-basic":
		return &auth.HTTPBasic{Prompter: c.promptSecret}, nil
	case "oauth-client-credentials":
		return &auth.ClientCredentials{
			Cache:      auth.NewTokenCache(c.tokenCachePath()),
			HTTPClient: &http.Client{Transport: c.baseHTTPTransport()},
		}, nil
	case "oauth-authorization-code":
		return &auth.AuthorizationCode{
			Cache:      auth.NewTokenCache(c.tokenCachePath()),
			HTTPClient: &http.Client{Transport: c.baseHTTPTransport()},
			Stderr:     c.Stderr,
		}, nil
	case "external-tool":
		return &auth.ExternalTool{}, nil
	default:
		return nil, fmt.Errorf("unknown auth type %q; supported: http-basic, oauth-client-credentials, oauth-authorization-code, external-tool", ac.Type)
	}
}

// authOnRequest returns an OnRequest hook for the given profile's auth config,
// or nil when no auth is configured.
// apiName and profileName are injected into params as "_cache_key".
func (c *CLI) authOnRequest(apiName, profileName string, prof *config.ProfileConfig) func(*http.Request) error {
	// Determine whether built-in auth is configured.
	var builtinOnReq func(*http.Request) error
	if prof != nil && prof.Auth != nil {
		handler, err := c.authHandlerFor(prof.Auth)
		if err != nil {
			return func(*http.Request) error { return err }
		}
		params := c.buildAuthParams(apiName, profileName, prof.Auth.Params)
		rawParams := prof.Auth.Params
		secretKeys := make(map[string]bool)
		for _, p := range handler.Parameters() {
			if p.Secret {
				secretKeys[p.Name] = true
			}
		}
		builtinOnReq = func(req *http.Request) error {
			if err := handler.OnRequest(req, params); err != nil {
				return err
			}
			return c.runAuthHookPlugins(apiName, profileName, rawParams, secretKeys, req)
		}
	}

	// Auth hook plugins run even when no built-in auth is configured.
	hookPlugins := c.pluginsForHook("auth")
	if builtinOnReq == nil && len(hookPlugins) == 0 {
		return nil
	}
	if builtinOnReq != nil {
		return builtinOnReq
	}
	// No built-in auth but hook plugins exist.
	return func(req *http.Request) error {
		return c.runAuthHookPlugins(apiName, profileName, nil, nil, req)
	}
}

// buildAuthParams returns a copy of rawParams with "_cache_key" injected.
func (c *CLI) buildAuthParams(apiName, profileName string, rawParams map[string]string) map[string]string {
	params := make(map[string]string, len(rawParams)+1)
	for k, v := range rawParams {
		params[k] = v
	}
	params["_cache_key"] = apiName + ":" + profileName
	return params
}

// tokenCachePath returns the effective path for the token cache file.
func (c *CLI) tokenCachePath() string {
	if c.TokenCachePath != "" {
		return c.TokenCachePath
	}
	return config.DefaultTokenCachePath()
}

// promptSecret writes prompt to Stderr then reads a secret.
// Uses PassReader when set (for tests); otherwise uses Stdin.
// When the source is a real terminal the input is not echoed.
func (c *CLI) promptSecret(prompt string) (string, error) {
	src := c.PassReader
	if src == nil {
		src = c.Stdin
	}
	return readPromptValue(prompt, src, c.Stderr, true)
}
