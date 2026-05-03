package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const defaultRedirectPort = "8484"
const authTimeout = 5 * time.Minute
const callbackPageResultWait = 200 * time.Millisecond

// AuthorizationCode implements the OAuth2 authorization code flow with PKCE
// (RFC 7636). On first use it opens a browser and waits for the redirect
// callback. Subsequent requests use the cached token; when expired the refresh
// token is used (if available), otherwise a new browser flow is started.
type AuthorizationCode struct {
	// Cache stores fetched tokens.
	Cache TokenStore
	// HTTPClient is used for token requests. Defaults to http.DefaultClient when nil.
	HTTPClient *http.Client
	// OpenBrowser is called with the authorization URL. When nil the default
	// system browser opener is used.
	OpenBrowser func(url string) error
	// Stderr receives status messages during the browser flow.
	Stderr io.Writer
	// Prompt is used to read a pasted authorization code for headless fallback.
	Prompt func(prompt string) (string, error)
	// CanPrompt reports whether manual code entry is safe for this invocation.
	CanPrompt bool
	// NoBrowser skips automatic browser launch and immediately falls back to
	// printing the auth URL for manual use.
	NoBrowser bool
	// Verbose prints the full authorization URL before browser launch.
	Verbose bool
}

func (h *AuthorizationCode) Parameters() []Param {
	return []Param{
		{Name: "client_id", Description: "OAuth2 client ID", Required: true},
		{Name: "client_secret", Description: "OAuth2 client secret (optional for public clients)", Required: false, Secret: true},
		{Name: "auth_method", Description: "OAuth2 client auth method: client_secret_post (default) or client_secret_basic", Required: false},
		{Name: "authorize_url", Description: "OAuth2 authorization endpoint URL", Required: false},
		{Name: "token_url", Description: "OAuth2 token endpoint URL", Required: false},
		{Name: "issuer_url", Description: "OIDC issuer URL (used for discovery when authorize_url/token_url are absent)", Required: false},
		{Name: "scopes", Description: "Space-separated OAuth2 scopes to request", Required: false},
		{Name: "redirect_port", Description: fmt.Sprintf("Local port for the redirect callback (default %s)", defaultRedirectPort), Required: false},
		{Name: "redirect_path", Description: "Local path for the redirect callback (default /)", Required: false},
	}
}

func (h *AuthorizationCode) OnRequest(req *http.Request, params map[string]string) error {
	return h.authenticateRequest(req, params, false)
}

func (h *AuthorizationCode) authenticateRequest(req *http.Request, params map[string]string, force bool) error {
	token, err := h.resolveToken(req.Context(), params, force)
	if err != nil {
		return err
	}
	bearerAuth(req, token)
	return nil
}

func (h *AuthorizationCode) resolveToken(ctx context.Context, params map[string]string, force bool) (string, error) {
	cacheKey := params["_cache_key"]
	var cached *CachedToken

	// Try cache first.
	if h.Cache != nil && cacheKey != "" {
		cachedToken, err := h.Cache.Get(cacheKey)
		if err == nil && cachedToken != nil {
			cached = cachedToken
			if !force && !cached.IsExpired() {
				return cached.AccessToken, nil
			}
			// Expired but has a refresh token — try to refresh.
			if cached.RefreshToken != "" {
				tokenURL, err := h.resolveTokenURL(ctx, params)
				if err != nil {
					return "", err
				}
				refreshed, err := h.doRefresh(ctx, params, tokenURL, cached.RefreshToken)
				if err == nil {
					if h.Cache != nil && cacheKey != "" {
						_ = h.Cache.Set(cacheKey, refreshed)
					}
					return refreshed.AccessToken, nil
				}
				if h.Stderr != nil {
					fmt.Fprintf(h.Stderr, "OAuth refresh failed: %v\n", err)
				}
				if !isTokenEndpointErrorCode(err, "invalid_grant") {
					return "", err
				}
				// Refresh token rejected — fall through to interactive auth.
			}
		}
	}

	// Full browser flow.
	authorizeURL, tokenURL, err := h.resolveEndpoints(ctx, params)
	if err != nil {
		return "", err
	}

	ct, err := h.doBrowserFlow(ctx, params, authorizeURL, tokenURL)
	if err != nil {
		return "", err
	}

	if h.Cache != nil && cacheKey != "" {
		_ = h.Cache.Set(cacheKey, ct)
	}
	return ct.AccessToken, nil
}

func (h *AuthorizationCode) Authenticate(ctx context.Context, req *http.Request, ac AuthContext) error {
	h2 := &AuthorizationCode{
		Cache:       h.Cache,
		HTTPClient:  h.HTTPClient,
		OpenBrowser: h.OpenBrowser,
		Stderr:      h.Stderr,
		Prompt:      h.Prompt,
		CanPrompt:   h.CanPrompt,
		NoBrowser:   h.NoBrowser,
		Verbose:     h.Verbose,
	}
	if ac.TokenStore != nil {
		h2.Cache = ac.TokenStore
	}
	if ac.HTTPClient != nil {
		h2.HTTPClient = ac.HTTPClient
	}
	if ac.Stderr != nil {
		h2.Stderr = ac.Stderr
	}
	if ac.Prompter != nil {
		h2.Prompt = ac.Prompter.Prompt
		h2.CanPrompt = true
	}
	return h2.authenticateRequest(req, authParams(ac), ac.Force)
}

func (h *AuthorizationCode) SupportsForce() {}

func (h *AuthorizationCode) resolveTokenURL(ctx context.Context, params map[string]string) (string, error) {
	if u := params["token_url"]; u != "" {
		if err := validateDirectOAuthEndpoint("token_url", u); err != nil {
			return "", err
		}
		return u, nil
	}
	_, tokenURL, err := h.resolveEndpoints(ctx, params)
	return tokenURL, err
}

func (h *AuthorizationCode) resolveEndpoints(ctx context.Context, params map[string]string) (authorizeURL, tokenURL string, err error) {
	authorizeURL = params["authorize_url"]
	tokenURL = params["token_url"]
	if authorizeURL != "" {
		if err := validateDirectOAuthEndpoint("authorize_url", authorizeURL); err != nil {
			return "", "", err
		}
	}
	if tokenURL != "" {
		if err := validateDirectOAuthEndpoint("token_url", tokenURL); err != nil {
			return "", "", err
		}
	}
	if authorizeURL == "" || tokenURL == "" {
		issuer := params["issuer_url"]
		if issuer == "" {
			return "", "", fmt.Errorf("oauth-authorization-code: (authorize_url and token_url) or issuer_url is required")
		}
		oidc, e := DiscoverOIDC(ctx, h.HTTPClient, issuer)
		if e != nil {
			return "", "", e
		}
		if e := validateOIDCEndpoints(issuer, oidc); e != nil {
			return "", "", e
		}
		if authorizeURL == "" {
			authorizeURL = oidc.AuthorizationEndpoint
		}
		if tokenURL == "" {
			tokenURL = oidc.TokenEndpoint
		}
	}
	return authorizeURL, tokenURL, nil
}

func (h *AuthorizationCode) doRefresh(ctx context.Context, params map[string]string, tokenURL, refreshToken string) (CachedToken, error) {
	return refreshOAuthToken(ctx, h.HTTPClient, params, tokenURL, refreshToken)
}

func (h *AuthorizationCode) doBrowserFlow(ctx context.Context, params map[string]string, authorizeURL, tokenURL string) (CachedToken, error) {
	// PKCE.
	verifier, err := generateCodeVerifier()
	if err != nil {
		return CachedToken{}, fmt.Errorf("generating PKCE verifier: %w", err)
	}
	challenge := codeChallenge(verifier)

	// State to prevent CSRF.
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return CachedToken{}, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Determine redirect port and URL.
	port := params["redirect_port"]
	if port == "" {
		port = defaultRedirectPort
	}
	redirectPath, err := oauthRedirectPath(params["redirect_path"])
	if err != nil {
		return CachedToken{}, err
	}
	redirectURI := "http://localhost:" + port + redirectPath

	// Build authorization URL.
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {params["client_id"]},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	if scopes := params["scopes"]; scopes != "" {
		q.Set("scope", scopes)
	}
	for key, value := range extraOAuthParams(params, map[string]bool{
		"_cache_key":    true,
		"authorize_url": true,
		"cache_key":     true,
		"issuer_url":    true,
		"redirect_path": true,
		"redirect_port": true,
		"token_url":     true,
	}) {
		if q.Get(key) == "" {
			q.Set(key, value)
		}
	}
	fullAuthorizeURL := authorizeURL + "?" + q.Encode()

	ctx2, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	var (
		codeCh chan string
		errCh  chan error
		doneCh chan error
		srv    *http.Server
	)
	manualOnly := h.CanPrompt && h.NoBrowser && h.Prompt != nil
	if !manualOnly {
		codeCh = make(chan string, 1)
		errCh = make(chan error, 1)
		doneCh = make(chan error, 1)
		var receivedCode atomic.Bool
		ln, err := net.Listen("tcp", "localhost:"+port)
		if err != nil {
			return CachedToken{}, fmt.Errorf("starting callback server on port %s: %w", port, err)
		}
		srv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != redirectPath {
				http.NotFound(w, r)
				return
			}

			q := r.URL.Query()
			if q.Get("state") != state {
				http.Error(w, "state mismatch", http.StatusBadRequest)
				trySendErr(errCh, fmt.Errorf("state mismatch in callback"))
				return
			}
			code := q.Get("code")
			if code == "" {
				http.Error(w, "missing code", http.StatusBadRequest)
				trySendErr(errCh, fmt.Errorf("no code in callback"))
				return
			}
			if !receivedCode.CompareAndSwap(false, true) {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, "<html><body><h2>Authentication already received</h2><p>You can close this tab.</p></body></html>")
				return
			}
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body><h2>Authorization code received</h2><p>Return to the terminal while Restish finishes authentication.</p></body></html>")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			select {
			case codeCh <- code:
			default:
			}
			select {
			case exchangeErr := <-doneCh:
				if exchangeErr != nil {
					fmt.Fprintf(w, "<html><body><h2>Authentication failed</h2><p>%s</p></body></html>", html.EscapeString(exchangeErr.Error()))
					return
				}
				fmt.Fprint(w, "<html><body><h2>Authentication successful</h2><p>You can close this tab.</p></body></html>")
			case <-time.After(callbackPageResultWait):
			case <-ctx2.Done():
				fmt.Fprint(w, "<html><body><h2>Authentication timed out</h2></body></html>")
			}
		})}
		go func() {
			if e := srv.Serve(ln); e != nil && e != http.ErrServerClosed {
				trySendErr(errCh, e)
			}
		}()
		defer func() {
			ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx2)
		}()
	}

	// Notify user and open browser. The full URL can contain sensitive request
	// parameters, so keep it out of stderr unless it is needed for manual action.
	if h.Stderr != nil && h.Verbose {
		fmt.Fprintf(h.Stderr, "Opening browser for authentication:\n  %s\n", fullAuthorizeURL)
	} else if h.Stderr != nil {
		fmt.Fprintln(h.Stderr, "Opening browser for authentication.")
	}

	var openErr error
	if !h.NoBrowser {
		opener := h.OpenBrowser
		if opener == nil {
			opener = DefaultOpenBrowser
		}
		if err := opener(fullAuthorizeURL); err != nil {
			openErr = err
			if h.Stderr != nil {
				fmt.Fprintf(h.Stderr, "Could not open browser: %v\nPlease open this URL manually:\n  %s\n", err, fullAuthorizeURL)
			}
		}
	} else if h.Stderr != nil {
		fmt.Fprintf(h.Stderr, "Browser launch disabled; open this URL manually:\n  %s\n", fullAuthorizeURL)
	}

	// Wait for callback.
	var code string
	if h.CanPrompt && (h.NoBrowser || openErr != nil) && h.Prompt != nil {
		promptCode, promptErr := h.Prompt("Paste the authorization code: ")
		if promptErr != nil {
			return CachedToken{}, promptErr
		}
		code = strings.TrimSpace(promptCode)
	} else {
		select {
		case code = <-codeCh:
		case err = <-errCh:
			return CachedToken{}, fmt.Errorf("callback error: %w", err)
		case <-ctx2.Done():
			return CachedToken{}, fmt.Errorf("timed out waiting for authorization callback")
		}
	}

	// Exchange code for token.
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {params["client_id"]},
		"code_verifier": {verifier},
	}
	ct, err := FetchToken(ctx, h.HTTPClient, tokenURL, form, params)
	if doneCh != nil {
		trySendErr(doneCh, err)
	}
	if err != nil {
		return CachedToken{}, err
	}
	return ct, nil
}

func oauthRedirectPath(value string) (string, error) {
	if value == "" {
		return "/", nil
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("redirect_path: invalid path %q: %w", value, err)
	}
	if u.IsAbs() || u.Host != "" || u.Scheme != "" {
		return "", fmt.Errorf("redirect_path: must be a local absolute path, not a URL")
	}
	if !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("redirect_path: must start with /")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("redirect_path: must not include query string or fragment")
	}
	return value, nil
}

func trySendErr(errCh chan error, err error) {
	select {
	case errCh <- err:
	default:
	}
}

// generateCodeVerifier returns a random PKCE code verifier (32 random bytes,
// base64url-encoded without padding).
func generateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// codeChallenge computes the S256 PKCE code challenge for the given verifier.
func codeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// DefaultOpenBrowser opens url in the system default browser.
func DefaultOpenBrowser(rawURL string) error {
	cmd := openBrowserCommand(rawURL)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

var openBrowserCommand = defaultOpenBrowserCommand

func defaultOpenBrowserCommand(rawURL string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "--", rawURL)
	case "windows":
		// Pass an explicit empty title ("") before the URL so that special
		// characters in the URL are not misinterpreted as window title or
		// cmd /c start flags.
		return exec.Command("cmd", "/c", "start", "", "--", rawURL)
	default:
		return exec.Command("xdg-open", "--", rawURL)
	}
}

// FreePort returns an available local TCP port as a string.
// Exported for use in integration tests.
func FreePort() (string, error) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()
	return port, nil
}

// TriggerCallback makes a GET request to the local callback server with the
// given port, code, and state. Used in tests to simulate the browser redirect.
func TriggerCallback(port, code, state string) error {
	u := "http://localhost:" + port + "/?code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(state)
	resp, err := http.Get(u) //nolint:gosec
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("callback returned %d", resp.StatusCode)
	}
	return nil
}

// StateFrom extracts the state parameter from a full authorization URL.
// Exported for use in integration tests.
func StateFrom(authURL string) string {
	u, err := url.Parse(authURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("state")
}

// RedirectPortFrom extracts the port from the redirect_uri param of a
// full authorization URL. Exported for use in integration tests.
func RedirectPortFrom(authURL string) string {
	u, err := url.Parse(authURL)
	if err != nil {
		return ""
	}
	ru, err := url.Parse(u.Query().Get("redirect_uri"))
	if err != nil {
		return ""
	}
	_, port, _ := net.SplitHostPort(ru.Host)
	return port
}
