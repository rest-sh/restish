package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

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

type resolvedAuthConfig struct {
	Config   *config.AuthConfig
	Ref      string
	CacheKey string
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
	resolvedAuth, err := c.resolveProfileAuth(apiName, profileName, prof)
	if err != nil {
		return err
	}
	if resolvedAuth.Config == nil {
		return fmt.Errorf("profile %q of API %q has no auth config", profileName, apiName)
	}

	authOpts, err := c.authHandlerOptionsFromCmd(cmd)
	if err != nil {
		return err
	}
	handler, err := c.authHandlerFor(resolvedAuth.Config, authOpts)
	if err != nil {
		return err
	}

	params, err := c.buildAuthParams(resolvedAuth.Config.Params)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := handler.Authenticate(requestContext(cmd), req, c.authContext(requestContext(cmd), apiName, profileName, params, resolvedAuth.CacheKey, false)); err != nil {
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
	resolvedAuth, err := c.resolveProfileAuth(apiName, profileName, prof)
	if err != nil {
		callbacks.OnRequest = func(*http.Request) error { return err }
		return callbacks
	}
	if resolvedAuth.Config != nil {
		handler, err := c.authHandlerFor(resolvedAuth.Config, opts)
		if err != nil {
			callbacks.OnRequest = func(*http.Request) error { return err }
			return callbacks
		}
		rawParams := resolvedAuth.Config.Params
		secretKeys := make(map[string]bool)
		for _, p := range handler.Parameters() {
			if p.Secret {
				secretKeys[p.Name] = true
			}
		}
		callbacks.OnRequest = func(req *http.Request) error {
			params, err := c.buildAuthParams(rawParams)
			if err != nil {
				return err
			}
			if resolvedAuth.Config.Type == "external-tool" {
				if err := c.ensureExternalToolApproved(req.Context(), apiName, profileName, params["commandline"]); err != nil {
					return err
				}
			}
			if err := handler.Authenticate(req.Context(), req, c.authContext(req.Context(), apiName, profileName, params, resolvedAuth.CacheKey, false)); err != nil {
				return err
			}
			return c.runAuthHookPlugins(apiName, profileName, rawParams, secretKeys, req)
		}
		if _, ok := handler.(auth.ForceCapable); ok {
			callbacks.OnUnauthorized = func(req *http.Request) error {
				params, err := c.buildAuthParams(rawParams)
				if err != nil {
					return err
				}
				if resolvedAuth.Config.Type == "external-tool" {
					if err := c.ensureExternalToolApproved(req.Context(), apiName, profileName, params["commandline"]); err != nil {
						return err
					}
				}
				if err := handler.Authenticate(req.Context(), req, c.authContext(req.Context(), apiName, profileName, params, resolvedAuth.CacheKey, true)); err != nil {
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

func (c *CLI) resolveProfileAuth(apiName, profileName string, prof *config.ProfileConfig) (resolvedAuthConfig, error) {
	if prof == nil {
		return resolvedAuthConfig{}, nil
	}
	if prof.Auth != nil && prof.AuthRef != "" {
		return resolvedAuthConfig{}, fmt.Errorf("profile %q of API %q has both auth and auth_ref", profileName, apiName)
	}
	if prof.AuthRef == "" {
		return resolvedAuthConfig{Config: prof.Auth}, nil
	}
	if c.cfg == nil || c.cfg.AuthProfiles == nil || c.cfg.AuthProfiles[prof.AuthRef] == nil {
		return resolvedAuthConfig{}, fmt.Errorf("profile %q of API %q references unknown auth profile %q", profileName, apiName, prof.AuthRef)
	}
	ac := c.cfg.AuthProfiles[prof.AuthRef]
	return resolvedAuthConfig{
		Config:   ac,
		Ref:      prof.AuthRef,
		CacheKey: sharedAuthCacheKey(prof.AuthRef, ac),
	}, nil
}

// buildAuthParams returns a copy of the user-supplied auth params with late
// secret sources resolved.
func (c *CLI) buildAuthParams(rawParams map[string]string) (map[string]string, error) {
	params := make(map[string]string, len(rawParams))
	for k, v := range rawParams {
		resolved, err := c.resolveAuthParam(v)
		if err != nil {
			return nil, fmt.Errorf("auth param %q: %w", k, err)
		}
		params[k] = resolved
	}
	return params, nil
}

func (c *CLI) resolveAuthParam(value string) (string, error) {
	switch {
	case strings.HasPrefix(value, "env:"):
		name := strings.TrimPrefix(value, "env:")
		if name == "" {
			return "", fmt.Errorf("env secret source is missing a variable name")
		}
		resolved, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("environment variable %s is not set", name)
		}
		return resolved, nil
	case strings.HasPrefix(value, "command:"):
		commandLine := strings.TrimSpace(strings.TrimPrefix(value, "command:"))
		if commandLine == "" {
			return "", fmt.Errorf("command secret source is empty")
		}
		return c.runSecretCommand(commandLine)
	default:
		return value, nil
	}
}

func (c *CLI) runSecretCommand(commandLine string) (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-c", commandLine)
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, limit: 4096}
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("secret command timed out or was canceled")
	}
	if err != nil {
		excerpt := strings.TrimSpace(stderr.String())
		if excerpt != "" {
			return "", fmt.Errorf("secret command failed: %w: stderr: %s", err, redactDiagnosticSecretText(excerpt))
		}
		return "", fmt.Errorf("secret command failed: %w", err)
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func (c *CLI) authContext(ctx context.Context, apiName, profileName string, params map[string]string, cacheKey string, force bool) auth.AuthContext {
	return auth.AuthContext{
		APIName:     apiName,
		ProfileName: profileName,
		CacheKey:    cacheKey,
		Params:      params,
		TokenStore:  auth.NewTokenCache(c.tokenCachePath()),
		Prompter:    cliPrompter{cli: c},
		Stderr:      c.Stderr,
		HTTPClient:  c.authHTTPClient(ctx),
		Logger:      log.New(c.Stderr, "", 0),
		Force:       force,
	}
}

func sharedAuthCacheKey(ref string, ac *config.AuthConfig) string {
	if ac == nil {
		return ""
	}
	if key := ac.Params["cache_key"]; key != "" {
		return "auth_profile:" + ref + ":" + key
	}
	relevant := map[string]string{"type": ac.Type}
	for _, name := range []string{"audience", "authorize_url", "client_id", "device_authorization_url", "issuer_url", "resource", "scopes", "token_url"} {
		if value := ac.Params[name]; value != "" {
			relevant[name] = value
		}
	}
	var keys []string
	for key := range relevant {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(relevant[key])
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "auth_profile:" + ref + ":" + hex.EncodeToString(sum[:8])
}

type limitedWriter struct {
	w     *bytes.Buffer
	limit int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.w != nil && w.limit > w.w.Len() {
		remaining := w.limit - w.w.Len()
		if remaining > len(p) {
			remaining = len(p)
		}
		if remaining > 0 {
			_, _ = w.w.Write(p[:remaining])
		}
	}
	return len(p), nil
}

func redactDiagnosticSecretText(value string) string {
	value = strings.TrimSpace(value)
	for _, marker := range []string{"access_token", "refresh_token", "id_token", "client_secret", "password", "authorization"} {
		value = redactDiagnosticAssignment(value, marker)
	}
	return value
}

func redactDiagnosticAssignment(value, marker string) string {
	for _, sep := range []string{"=", ":"} {
		lower := strings.ToLower(value)
		needle := strings.ToLower(marker + sep)
		searchFrom := 0
		for {
			idxRel := strings.Index(lower[searchFrom:], needle)
			if idxRel < 0 {
				break
			}
			idx := searchFrom + idxRel
			start := idx + len(needle)
			end := start
			for end < len(value) && !strings.ContainsRune(" \t\r\n,;&", rune(value[end])) {
				end++
			}
			value = value[:start] + "***" + value[end:]
			lower = strings.ToLower(value)
			searchFrom = start + len("***")
		}
	}
	return value
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
