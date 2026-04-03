package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

const defaultRedirectPort = "8484"
const authTimeout = 5 * time.Minute

// AuthorizationCode implements the OAuth2 authorization code flow with PKCE
// (RFC 7636). On first use it opens a browser and waits for the redirect
// callback. Subsequent requests use the cached token; when expired the refresh
// token is used (if available), otherwise a new browser flow is started.
type AuthorizationCode struct {
	// Cache stores fetched tokens.
	Cache *TokenCache
	// HTTPClient is used for token requests. Defaults to http.DefaultClient when nil.
	HTTPClient *http.Client
	// OpenBrowser is called with the authorization URL. When nil the default
	// system browser opener is used.
	OpenBrowser func(url string) error
	// Stderr receives status messages during the browser flow.
	Stderr io.Writer
}

func (h *AuthorizationCode) Parameters() []Param {
	return []Param{
		{Name: "client_id", Description: "OAuth2 client ID", Required: true},
		{Name: "client_secret", Description: "OAuth2 client secret (optional for public clients)", Required: false, Secret: true},
		{Name: "authorize_url", Description: "OAuth2 authorization endpoint URL", Required: false},
		{Name: "token_url", Description: "OAuth2 token endpoint URL", Required: false},
		{Name: "issuer_url", Description: "OIDC issuer URL (used for discovery when authorize_url/token_url are absent)", Required: false},
		{Name: "scopes", Description: "Space-separated OAuth2 scopes to request", Required: false},
		{Name: "redirect_port", Description: fmt.Sprintf("Local port for the redirect callback (default %s)", defaultRedirectPort), Required: false},
	}
}

func (h *AuthorizationCode) OnRequest(req *http.Request, params map[string]string) error {
	token, err := h.resolveToken(req.Context(), params)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (h *AuthorizationCode) resolveToken(ctx context.Context, params map[string]string) (string, error) {
	cacheKey := params["_cache_key"]

	// Try cache first.
	if h.Cache != nil && cacheKey != "" {
		cached, err := h.Cache.Get(cacheKey)
		if err == nil && cached != nil {
			if !cached.IsExpired() {
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
				// Refresh failed — fall through to browser flow.
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

func (h *AuthorizationCode) resolveTokenURL(ctx context.Context, params map[string]string) (string, error) {
	if u := params["token_url"]; u != "" {
		return u, nil
	}
	if issuer := params["issuer_url"]; issuer != "" {
		oidc, err := DiscoverOIDC(ctx, h.HTTPClient, issuer)
		if err != nil {
			return "", err
		}
		return oidc.TokenEndpoint, nil
	}
	return "", fmt.Errorf("oauth-authorization-code: token_url or issuer_url is required")
}

func (h *AuthorizationCode) resolveEndpoints(ctx context.Context, params map[string]string) (authorizeURL, tokenURL string, err error) {
	authorizeURL = params["authorize_url"]
	tokenURL = params["token_url"]
	if authorizeURL == "" || tokenURL == "" {
		issuer := params["issuer_url"]
		if issuer == "" {
			return "", "", fmt.Errorf("oauth-authorization-code: (authorize_url and token_url) or issuer_url is required")
		}
		oidc, e := DiscoverOIDC(ctx, h.HTTPClient, issuer)
		if e != nil {
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
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {params["client_id"]},
	}
	if cs := params["client_secret"]; cs != "" {
		form.Set("client_secret", cs)
	}
	return FetchToken(ctx, h.HTTPClient, tokenURL, form.Encode())
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
	redirectURI := "http://localhost:" + port

	// Start local callback server.
	ln, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		return CachedToken{}, fmt.Errorf("starting callback server on port %s: %w", port, err)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("state mismatch in callback")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("no code in callback")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Authentication successful</h2><p>You can close this tab.</p></body></html>")
		codeCh <- code
	})}

	go func() {
		if e := srv.Serve(ln); e != nil && e != http.ErrServerClosed {
			errCh <- e
		}
	}()
	defer func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx2)
	}()

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
	fullAuthorizeURL := authorizeURL + "?" + q.Encode()

	// Notify user and open browser.
	if h.Stderr != nil {
		fmt.Fprintf(h.Stderr, "Opening browser for authentication:\n  %s\n", fullAuthorizeURL)
	}
	opener := h.OpenBrowser
	if opener == nil {
		opener = DefaultOpenBrowser
	}
	if err := opener(fullAuthorizeURL); err != nil && h.Stderr != nil {
		fmt.Fprintf(h.Stderr, "Could not open browser: %v\nPlease open the URL above manually.\n", err)
	}

	// Wait for callback.
	ctx2, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return CachedToken{}, fmt.Errorf("callback error: %w", err)
	case <-ctx2.Done():
		return CachedToken{}, fmt.Errorf("timed out waiting for authorization callback")
	}

	// Exchange code for token.
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {params["client_id"]},
		"code_verifier": {verifier},
	}
	if cs := params["client_secret"]; cs != "" {
		form.Set("client_secret", cs)
	}
	return FetchToken(ctx, h.HTTPClient, tokenURL, form.Encode())
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
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}

// freePort returns the string representation of an available local TCP port.
// Used in tests to avoid hard-coding port 8484.
func freePort() (string, error) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return "", err
	}
	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	_ = ln.Close()
	return port, nil
}

// FreePort returns an available local TCP port as a string.
// Exported for use in integration tests.
func FreePort() (string, error) {
	return freePort()
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

// stateFrom extracts the state parameter from a full authorization URL.
// Used in tests to capture the state generated by the handler.
func stateFrom(authURL string) string {
	u, err := url.Parse(authURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("state")
}

// StateFrom exports stateFrom for tests.
func StateFrom(authURL string) string {
	return stateFrom(authURL)
}

// redirectPortFrom extracts the port from the redirect_uri param of a
// full authorization URL. Used in tests.
func redirectPortFrom(authURL string) string {
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

// RedirectPortFrom exports redirectPortFrom for tests.
func RedirectPortFrom(authURL string) string {
	return redirectPortFrom(authURL)
}

