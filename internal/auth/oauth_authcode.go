package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
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
const defaultCallbackSuccessColor = "#5fafd7"
const defaultCallbackFailureColor = "#E94F37"
const callbackSuccessHTMLParam = "callback_success_html"
const callbackErrorHTMLParam = "callback_error_html"
const defaultRedirectScheme = "http"

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
	// CallbackSuccessColor customizes the browser callback success background.
	// Invalid values fall back to the built-in v1 callback color.
	CallbackSuccessColor string
	// CallbackFailureColor customizes the browser callback failure background.
	// Invalid values fall back to the built-in v1 callback color.
	CallbackFailureColor string
	// CallbackSuccessHTML customizes the browser callback success page. When
	// empty, Restish renders the built-in animated page.
	CallbackSuccessHTML string
	// CallbackErrorHTML customizes the browser callback failure page. When
	// empty, Restish renders the built-in animated page. $ERROR, $TITLE, and
	// $DETAILS placeholders are replaced with escaped callback values.
	CallbackErrorHTML string
}

func (h *AuthorizationCode) Parameters() []Param {
	return appendOAuthPassthroughParams([]Param{
		{Name: "client_id", Description: "OAuth2 client ID", Required: true},
		{Name: "client_secret", Description: "OAuth2 client secret (optional for public clients)", Required: false, Secret: true},
		{Name: "auth_method", Description: "OAuth2 client auth method: client_secret_post (default) or client_secret_basic", Required: false},
		{Name: "authorize_url", Description: "OAuth2 authorization endpoint URL", Required: false},
		{Name: "token_url", Description: "OAuth2 token endpoint URL", Required: false},
		{Name: "issuer_url", Description: "OIDC issuer URL (used for discovery when authorize_url/token_url are absent)", Required: false},
		{Name: "scopes", Description: "Space-separated OAuth2 scopes to request; some providers require offline_access for refresh tokens", Required: false},
		{Name: "redirect_scheme", Description: "Local callback URL scheme: http (default) or https", Required: false},
		{Name: "redirect_port", Description: fmt.Sprintf("Local port for the redirect callback (default %s)", defaultRedirectPort), Required: false},
		{Name: "redirect_path", Description: "Local path for the redirect callback (default /)", Required: false},
		{Name: "redirect_cert", Description: "Path to the PEM certificate for an HTTPS local callback", Required: false},
		{Name: "redirect_key", Description: "Path to the PEM private key for an HTTPS local callback", Required: false, Secret: true},
		{Name: callbackSuccessHTMLParam, Description: "Custom HTML for the successful browser callback page", Required: false},
		{Name: callbackErrorHTMLParam, Description: "Custom HTML for the failed browser callback page; supports $ERROR, $TITLE, and $DETAILS placeholders", Required: false},
	})
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

	// Try cache first.
	var tokenURL string
	var tokenURLErr error
	token, ok, err := cachedOAuthAccessToken(h.Cache, cacheKey, force, func(cached CachedToken) (CachedToken, error) {
		if tokenURL == "" && tokenURLErr == nil {
			tokenURL, tokenURLErr = h.resolveTokenURL(ctx, params)
		}
		if tokenURLErr != nil {
			return CachedToken{}, tokenURLErr
		}
		return h.doRefresh(ctx, params, tokenURL, cached.RefreshToken)
	})
	if ok {
		return token, nil
	}
	if err != nil {
		if h.Stderr != nil {
			fmt.Fprintf(h.Stderr, "OAuth refresh failed: %v\n", err)
		}
		if !isTokenEndpointErrorCode(err, "invalid_grant") {
			return "", err
		}
		clearRejectedOAuthToken(h.Cache, cacheKey, h.Stderr)
		// Refresh token rejected — fall through to interactive auth.
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
	warnIfMissingOAuthRefreshToken(h.Stderr, params, ct)
	return ct.AccessToken, nil
}

func (h *AuthorizationCode) Authenticate(ctx context.Context, req *http.Request, ac AuthContext) error {
	h2 := &AuthorizationCode{
		Cache:                h.Cache,
		HTTPClient:           h.HTTPClient,
		OpenBrowser:          h.OpenBrowser,
		Stderr:               h.Stderr,
		Prompt:               h.Prompt,
		CanPrompt:            h.CanPrompt,
		NoBrowser:            h.NoBrowser,
		Verbose:              h.Verbose,
		CallbackSuccessColor: h.CallbackSuccessColor,
		CallbackFailureColor: h.CallbackFailureColor,
		CallbackSuccessHTML:  h.CallbackSuccessHTML,
		CallbackErrorHTML:    h.CallbackErrorHTML,
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
	if ac.Prompter != nil && h2.CanPrompt {
		h2.Prompt = ac.Prompter.Prompt
	}
	req = requestWithContext(req, ctx)
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

	manualOnly := h.CanPrompt && h.NoBrowser && h.Prompt != nil

	// Determine redirect URL.
	redirect, err := oauthRedirectConfigFromParams(params, !manualOnly)
	if err != nil {
		return CachedToken{}, err
	}
	callbackPages := h.oauthCallbackPages(params)

	// Build authorization URL.
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {params["client_id"]},
		"redirect_uri":          {redirect.uri},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	if scopes := params["scopes"]; scopes != "" {
		q.Set("scope", scopes)
	}
	for key, value := range extraOAuthParams(params, map[string]bool{
		"_cache_key":             true,
		"authorize_url":          true,
		"cache_key":              true,
		"issuer_url":             true,
		callbackErrorHTMLParam:   true,
		callbackSuccessHTMLParam: true,
		"redirect_path":          true,
		"redirect_port":          true,
		"redirect_scheme":        true,
		"redirect_cert":          true,
		"redirect_key":           true,
		"token_url":              true,
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
	if !manualOnly {
		codeCh = make(chan string, 1)
		errCh = make(chan error, 1)
		doneCh = make(chan error, 1)
		var receivedCode atomic.Bool
		ln, err := net.Listen("tcp", "localhost:"+redirect.port)
		if err != nil {
			return CachedToken{}, fmt.Errorf("starting callback server on port %s: %w", redirect.port, err)
		}
		srv = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != redirect.path {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			q := r.URL.Query()
			if q.Get("state") != state {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, callbackPages.errorPage("Authentication failed", "State mismatch in OAuth callback. Return to the terminal to try again.", "state_mismatch"))
				trySendErr(errCh, fmt.Errorf("state mismatch in callback"))
				return
			}
			if callbackErr := q.Get("error"); callbackErr != "" {
				detail := q.Get("error_description")
				if detail == "" {
					detail = "The OAuth provider rejected the authorization request."
				}
				fmt.Fprint(w, callbackPages.errorPage("Error: "+callbackErr, detail, callbackErr))
				trySendErr(errCh, fmt.Errorf("oauth callback error: %s", callbackErr))
				return
			}
			code := q.Get("code")
			if code == "" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, callbackPages.errorPage("Authentication failed", "No authorization code was included in the OAuth callback.", "missing_code"))
				trySendErr(errCh, fmt.Errorf("no code in callback"))
				return
			}
			if !receivedCode.CompareAndSwap(false, true) {
				fmt.Fprint(w, callbackPages.successPage("Authentication already received", "You can close this tab."))
				return
			}
			select {
			case codeCh <- code:
			default:
			}
			select {
			case exchangeErr := <-doneCh:
				if exchangeErr != nil {
					fmt.Fprint(w, callbackPages.errorPage("Authentication failed", exchangeErr.Error(), "token_exchange_failed"))
					return
				}
				fmt.Fprint(w, callbackPages.successPage("Login Successful!", "Please return to the terminal. You may now close this window."))
			case <-time.After(callbackPageResultWait):
				fmt.Fprint(w, callbackPages.successPage("Authorization code received", "Return to the terminal while Restish finishes authentication."))
			case <-ctx2.Done():
				fmt.Fprint(w, callbackPages.errorPage("Authentication timed out", "Return to the terminal to try again.", "timed_out"))
			}
		})}
		go func() {
			if e := serveOAuthCallback(srv, ln, redirect); e != nil && e != http.ErrServerClosed {
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
			if errors.Is(ctx2.Err(), context.Canceled) {
				return CachedToken{}, ctx2.Err()
			}
			return CachedToken{}, fmt.Errorf("timed out waiting for authorization callback")
		}
	}

	// Exchange code for token.
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirect.uri},
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

type oauthRedirectConfig struct {
	scheme string
	port   string
	path   string
	uri    string
	cert   string
	key    string
}

func oauthRedirectConfigFromParams(params map[string]string, requireTLSFiles bool) (oauthRedirectConfig, error) {
	scheme := strings.ToLower(strings.TrimSpace(params["redirect_scheme"]))
	if scheme == "" {
		scheme = defaultRedirectScheme
	}
	if scheme != "http" && scheme != "https" {
		return oauthRedirectConfig{}, fmt.Errorf("oauth-authorization-code: redirect_scheme must be http or https")
	}
	cert := params["redirect_cert"]
	key := params["redirect_key"]
	if scheme == "https" && requireTLSFiles && (cert == "" || key == "") {
		return oauthRedirectConfig{}, fmt.Errorf("oauth-authorization-code: redirect_cert and redirect_key are required when redirect_scheme is https")
	}
	port := params["redirect_port"]
	if port == "" {
		port = defaultRedirectPort
	}
	path, err := oauthRedirectPath(params["redirect_path"])
	if err != nil {
		return oauthRedirectConfig{}, err
	}
	return oauthRedirectConfig{
		scheme: scheme,
		port:   port,
		path:   path,
		uri:    scheme + "://localhost:" + port + path,
		cert:   cert,
		key:    key,
	}, nil
}

func serveOAuthCallback(srv *http.Server, ln net.Listener, redirect oauthRedirectConfig) error {
	if redirect.scheme == "https" {
		return srv.ServeTLS(ln, redirect.cert, redirect.key)
	}
	return srv.Serve(ln)
}

func (h *AuthorizationCode) oauthCallbackSuccessPage(title, detail string) string {
	return h.oauthCallbackPages(nil).successPage(title, detail)
}

func (h *AuthorizationCode) oauthCallbackErrorPage(title, detail string) string {
	return h.oauthCallbackPages(nil).errorPage(title, detail, "")
}

type oauthCallbackPages struct {
	successColor string
	failureColor string
	successHTML  string
	errorHTML    string
}

type oauthCallbackTemplateData struct {
	Title   string
	Details string
	Error   string
}

func (h *AuthorizationCode) oauthCallbackPages(params map[string]string) oauthCallbackPages {
	pages := oauthCallbackPages{
		successColor: h.CallbackSuccessColor,
		failureColor: h.CallbackFailureColor,
		successHTML:  h.CallbackSuccessHTML,
		errorHTML:    h.CallbackErrorHTML,
	}
	if params != nil {
		if html := params[callbackSuccessHTMLParam]; html != "" {
			pages.successHTML = html
		}
		if html := params[callbackErrorHTMLParam]; html != "" {
			pages.errorHTML = html
		}
	}
	return pages
}

func (p oauthCallbackPages) successPage(title, detail string) string {
	if p.successHTML != "" {
		return renderOAuthCallbackTemplate(p.successHTML, oauthCallbackTemplateData{
			Title:   title,
			Details: detail,
		})
	}
	return oauthCallbackSuccessPage(title, detail, p.successColor)
}

func (p oauthCallbackPages) errorPage(title, detail, errorCode string) string {
	if p.errorHTML != "" {
		return renderOAuthCallbackTemplate(p.errorHTML, oauthCallbackTemplateData{
			Title:   title,
			Details: detail,
			Error:   errorCode,
		})
	}
	return oauthCallbackErrorPage(title, detail, p.failureColor)
}

func renderOAuthCallbackTemplate(template string, data oauthCallbackTemplateData) string {
	errorText := data.Error
	if errorText == "" {
		errorText = data.Title
	}
	replacer := strings.NewReplacer(
		"$ERROR", html.EscapeString(errorText),
		"$TITLE", html.EscapeString(data.Title),
		"$DETAILS", html.EscapeString(data.Details),
	)
	return replacer.Replace(template)
}

func oauthCallbackSuccessPage(title, detail, color string) string {
	return oauthCallbackPage("success", title, detail, callbackPageColor(color, defaultCallbackSuccessColor))
}

func oauthCallbackErrorPage(title, detail, color string) string {
	return oauthCallbackPage("failure", title, detail, callbackPageColor(color, defaultCallbackFailureColor))
}

func oauthCallbackPage(kind, title, detail, background string) string {
	title = html.EscapeString(title)
	detail = html.EscapeString(detail)
	detailHTML := ""
	if detail != "" {
		detailHTML = "<p>" + detail + "</p>"
	}
	iconHTML := `<div class="check"></div>`
	if kind == "failure" {
		iconHTML = `<div class="x-wrap"><div class="x"></div></div>`
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Restish OAuth</title>
    <style>
      @keyframes success-bg {
        from { background: white; }
        to { background: %s; }
      }
      @keyframes failure-bg {
        from { background: white; }
        to { background: %s; }
      }
      @keyframes check {
        from { transform: rotate(0deg) skew(30deg, 20deg); }
        to { transform: rotate(-45deg); }
      }
      @keyframes x {
        from { transform: scaleY(0); }
        to { transform: scaleY(1) rotate(-90deg); }
      }
      @keyframes fade {
        from { opacity: 0; }
        to { opacity: 1; }
      }
      html {
        min-height: 100%%;
      }
      body {
        min-height: 100vh;
        margin: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: sans-serif;
        animation-duration: 1.5s;
        animation-timing-function: ease-out;
        animation-fill-mode: forwards;
      }
      body.success {
        animation-name: success-bg;
      }
      body.failure {
        animation-name: failure-bg;
      }
      main {
        width: min(520px, calc(100%% - 48px));
        text-align: left;
      }
      .check {
        width: 160px;
        height: 96px;
        margin: 0 auto 76px;
        border-left: 16px solid white;
        border-bottom: 16px solid white;
        animation: check 0.7s cubic-bezier(0.175, 0.885, 0.32, 1.275);
        animation-fill-mode: forwards;
      }
      .x-wrap {
        margin: 0 auto 116px;
        transform: rotate(-45deg);
      }
      .x,
      .x:after {
        width: 180px;
        height: 16px;
        margin: auto;
        background: white;
        border-radius: 3px;
        transform: rotate(-45deg);
        animation: x 0.7s cubic-bezier(0.175, 0.885, 0.32, 1.275);
        animation-fill-mode: forwards;
      }
      .x:after {
        content: "";
        display: block;
        width: 100%%;
        transform: rotate(90deg);
      }
      .msg {
        background: white;
        padding: 20px 32px;
        border-radius: 10px;
        animation: fade 2s;
        animation-fill-mode: forwards;
        box-shadow: 0 15px 15px -15px rgba(0, 0, 0, 0.5);
      }
      h1 {
        margin: 0 0 12px;
        font-size: 32px;
        line-height: 1.15;
      }
      p {
        margin: 0;
        font-size: 16px;
        line-height: 1.5;
      }
      @media (prefers-reduced-motion: reduce) {
        body,
        .check,
        .x,
        .x:after,
        .msg {
          animation-duration: 1ms;
        }
      }
    </style>
  </head>
  <body class="%s">
    <main>
      %s
      <div class="msg">
        <h1>%s</h1>
        %s
      </div>
    </main>
  </body>
</html>`, background, background, kind, iconHTML, title, detailHTML)
}

func callbackPageColor(color, fallback string) string {
	color = strings.TrimSpace(color)
	if isCSSHexColor(color) {
		return color
	}
	return fallback
}

func isCSSHexColor(color string) bool {
	if len(color) != 4 && len(color) != 7 {
		return false
	}
	if color[0] != '#' {
		return false
	}
	for _, r := range color[1:] {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
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
