package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/internal/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/output"
	"github.com/rest-sh/restish/v2/internal/request"
	"github.com/spf13/cobra"
)

type authHandlerOptions struct {
	NoBrowser bool
	Verbose   bool
}

type authCallbacks struct {
	OnRequest      func(*http.Request) error
	OnUnauthorized func(*http.Request) error
}

type cliPrompter struct{ cli *CLI }

func (p cliPrompter) Prompt(prompt string) (string, error) {
	return p.cli.Prompt(context.Background(), prompt)
}
func (p cliPrompter) PromptSecret(prompt string) (string, error) {
	return p.cli.Secret(context.Background(), prompt)
}

// addAuthHeaderCommand registers the "auth-header" command on root.
func (c *CLI) addAuthHeaderCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:     "auth-header <api>",
		Short:   "Print the Authorization header value for a registered API",
		GroupID: rootGroupUtility,
		Args:    cobra.ExactArgs(1),
		RunE:    c.runAuthHeader,
	})
}

// runAuthHeader resolves auth for the named API and prints the Authorization
// header value.
func (c *CLI) runAuthHeader(cmd *cobra.Command, args []string) error {
	apiName := args[0]
	if c.cfg == nil || c.cfg.APIs[apiName] == nil {
		return fmt.Errorf("unknown API %q; run \"restish api list\" to see configured APIs", apiName)
	}
	api := c.cfg.APIs[apiName]

	profileName := c.profileFromCmd(cmd)

	if api.Profiles == nil || api.Profiles[profileName] == nil {
		return fmt.Errorf("API %q has no profile %q; configured profiles: %s", apiName, profileName, profileNames(api.Profiles))
	}
	prof := api.Profiles[profileName]
	if prof.Auth == nil {
		return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
	}

	authOpts, err := c.authHandlerOptionsFromCmd(cmd)
	if err != nil {
		return err
	}
	handler, err := c.authHandlerFor(prof.Auth, authOpts)
	if err != nil {
		return err
	}

	params := c.buildAuthParams(prof.Auth.Params)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := handler.Authenticate(requestContext(cmd), req, c.authContext(requestContext(cmd), apiName, profileName, params, false)); err != nil {
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
func (c *CLI) authHandlerOptionsFromCmd(cmd *cobra.Command) (authHandlerOptions, error) {
	if cmd == nil {
		return authHandlerOptions{}, nil
	}
	gf := globalFlagsFromContext(requestContext(cmd))
	return authHandlerOptions{NoBrowser: gf.NoBrowser, Verbose: gf.Verbose > 0}, nil
}

func (c *CLI) authHandlerFor(ac *config.AuthConfig, opts authHandlerOptions) (auth.Handler, error) {
	if c.customAuthHandlers != nil {
		if h, ok := c.customAuthHandlers[ac.Type]; ok {
			return h, nil
		}
	}
	switch ac.Type {
	case "http-basic":
		return &auth.HTTPBasic{Prompter: func(prompt string) (string, error) {
			return c.Secret(context.Background(), prompt)
		}}, nil
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
			Prompt: func(prompt string) (string, error) {
				return c.Prompt(context.Background(), prompt)
			},
			CanPrompt: c.canPromptCode(),
			NoBrowser: opts.NoBrowser,
			Verbose:   opts.Verbose,
		}, nil
	case "oauth-device-code":
		return &auth.DeviceCode{
			Cache:      auth.NewTokenCache(c.tokenCachePath()),
			HTTPClient: &http.Client{Transport: c.baseHTTPTransport()},
			Stderr:     c.Stderr,
		}, nil
	case "external-tool":
		return &auth.ExternalTool{Stderr: c.Stderr}, nil
	default:
		return nil, fmt.Errorf("unknown auth type %q; supported: http-basic, oauth-client-credentials, oauth-authorization-code, oauth-device-code, external-tool", ac.Type)
	}
}

// authOnRequest returns auth callbacks for the given profile's auth config,
// or zero values when no auth is configured.
func (c *CLI) authOnRequest(apiName, profileName string, prof *config.ProfileConfig, opts authHandlerOptions) authCallbacks {
	// Determine whether built-in auth is configured.
	var callbacks authCallbacks
	if prof != nil && prof.Auth != nil {
		handler, err := c.authHandlerFor(prof.Auth, opts)
		if err != nil {
			callbacks.OnRequest = func(*http.Request) error { return err }
			return callbacks
		}
		params := c.buildAuthParams(prof.Auth.Params)
		rawParams := prof.Auth.Params
		secretKeys := make(map[string]bool)
		for _, p := range handler.Parameters() {
			if p.Secret {
				secretKeys[p.Name] = true
			}
		}
		callbacks.OnRequest = func(req *http.Request) error {
			if prof.Auth.Type == "external-tool" {
				if err := c.ensureExternalToolApproved(req.Context(), apiName, profileName, params["commandline"]); err != nil {
					return err
				}
			}
			if err := handler.Authenticate(req.Context(), req, c.authContext(req.Context(), apiName, profileName, params, false)); err != nil {
				return err
			}
			return c.runAuthHookPlugins(apiName, profileName, rawParams, secretKeys, req)
		}
		if _, ok := handler.(auth.ForceCapable); ok {
			callbacks.OnUnauthorized = func(req *http.Request) error {
				if prof.Auth.Type == "external-tool" {
					if err := c.ensureExternalToolApproved(req.Context(), apiName, profileName, params["commandline"]); err != nil {
						return err
					}
				}
				if err := handler.Authenticate(req.Context(), req, c.authContext(req.Context(), apiName, profileName, params, true)); err != nil {
					return err
				}
				return c.runAuthHookPlugins(apiName, profileName, rawParams, secretKeys, req)
			}
		}
	}

	// Auth hook plugins run even when no built-in auth is configured.
	hookPlugins := c.pluginsByHook["auth"]
	if callbacks.OnRequest == nil && len(hookPlugins) == 0 {
		return callbacks
	}
	if callbacks.OnRequest != nil {
		return callbacks
	}
	// No built-in auth but hook plugins exist.
	callbacks.OnRequest = func(req *http.Request) error {
		return c.runAuthHookPlugins(apiName, profileName, nil, nil, req)
	}
	return callbacks
}

// buildAuthParams returns a copy of the user-supplied auth params.
func (c *CLI) buildAuthParams(rawParams map[string]string) map[string]string {
	params := make(map[string]string, len(rawParams))
	for k, v := range rawParams {
		params[k] = v
	}
	return params
}

func (c *CLI) authContext(ctx context.Context, apiName, profileName string, params map[string]string, force bool) auth.AuthContext {
	return auth.AuthContext{
		APIName:     apiName,
		ProfileName: profileName,
		Params:      params,
		TokenStore:  auth.NewTokenCache(c.tokenCachePath()),
		Prompter:    cliPrompter{cli: c},
		Stderr:      c.Stderr,
		HTTPClient:  c.authHTTPClient(ctx),
		Logger:      log.New(c.Stderr, "", 0),
		Force:       force,
	}
}

func (c *CLI) authHTTPClient(ctx context.Context) *http.Client {
	gf := globalFlagsFromContext(ctx)
	tlsMinVersion, _ := request.TLSVersionFromString(gf.TLSMinVersion)
	return &http.Client{Transport: request.BuildTransport(request.Options{
		Transport:     c.baseHTTPTransport(),
		Insecure:      gf.Insecure,
		CACertPath:    gf.CACert,
		TLSMinVersion: tlsMinVersion,
	})}
}

func profileNames(profiles map[string]*config.ProfileConfig) string {
	if len(profiles) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// tokenCachePath returns the effective path for the token cache file.
func (c *CLI) tokenCachePath() string {
	if c.hooks.TokenCachePath != "" {
		return c.hooks.TokenCachePath
	}
	return c.paths().TokenCache()
}

func (c *CLI) canPromptCode() bool {
	if c.hooks.PassReader != nil {
		return true
	}
	return output.IsTerminalReader(c.Stdin)
}
