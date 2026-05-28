package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/rest-sh/restish/v2/internal/secrets"
	"golang.org/x/net/idna"
)

const (
	authMethodClientSecretPost  = "client_secret_post"
	authMethodClientSecretBasic = "client_secret_basic"
	maxOAuthEndpointBodyBytes   = 1 << 20
)

func appendOAuthPassthroughParams(params []Param) []Param {
	return append(params,
		Param{Name: "audience", Description: "OAuth2 audience/resource server identifier to pass to the provider", Required: false},
		Param{Name: "resource", Description: "OAuth2 resource identifier to pass to providers that require it", Required: false},
		Param{Name: "organization", Description: "OAuth2 organization or tenant identifier to pass to providers that require it", Required: false},
	)
}

func clearRejectedOAuthToken(cache TokenStore, cacheKey string, stderr io.Writer) {
	if cache == nil || cacheKey == "" {
		return
	}
	if err := cache.Delete(cacheKey); err == nil && stderr != nil {
		fmt.Fprintln(stderr, "OAuth refresh token was rejected; cleared cached token")
	}
}

func warnIfMissingOAuthRefreshToken(stderr io.Writer, params map[string]string, token CachedToken) {
	if stderr == nil || params["_cache_key"] == "" || strings.TrimSpace(token.RefreshToken) != "" {
		return
	}
	fmt.Fprintln(stderr, "OAuth token response did not include a refresh token; some providers require an offline_access scope for long-lived sessions")
}

// tokenResponse is the JSON response from an OAuth2 token endpoint.
type tokenResponse struct {
	AccessToken  string          `json:"access_token"`
	TokenType    string          `json:"token_type"`
	ExpiresIn    secondsOrString `json:"expires_in"` // seconds; 0 means no expiry info
	RefreshToken string          `json:"refresh_token,omitempty"`
	Scope        string          `json:"scope,omitempty"`
}

type secondsOrString int

func (s *secondsOrString) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*s = secondsOrString(n)
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("expires_in must be a number or numeric string")
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("expires_in must be a number or numeric string")
	}
	*s = secondsOrString(parsed)
	return nil
}

// OIDCConfig holds the fields we use from an OIDC discovery document.
type OIDCConfig struct {
	AuthorizationEndpoint       string `json:"authorization_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

type tokenEndpointError struct {
	StatusCode  int
	ErrorCode   string
	Description string
	Body        string
}

func (e *tokenEndpointError) Error() string {
	msg := fmt.Sprintf("token endpoint returned %d", e.StatusCode)
	if e.ErrorCode != "" {
		msg += ": " + e.ErrorCode
	}
	if e.Description != "" {
		msg += " (" + e.Description + ")"
	} else if e.Body != "" {
		msg += ": " + e.Body
	}
	return msg
}

// DiscoverOIDC fetches issuerURL+"/.well-known/openid-configuration".
// Pass nil for client to use http.DefaultClient.
func DiscoverOIDC(ctx context.Context, client *http.Client, issuerURL string) (*OIDCConfig, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if err := validateOAuthIssuerURL(issuerURL); err != nil {
		return nil, err
	}
	discoveryURL := strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery from %s: %w", issuerURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("OIDC discovery from %s: unexpected status %d", issuerURL, resp.StatusCode)
	}
	body, err := readOAuthEndpointBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery from %s: %w", issuerURL, err)
	}
	var cfg OIDCConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, fmt.Errorf("OIDC discovery from %s: %w", issuerURL, err)
	}
	return &cfg, nil
}

func validateOAuthIssuerURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("issuer_url: invalid OAuth issuer URL %q: %w", rawURL, err)
	}
	if !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("issuer_url: OAuth issuer URL %q must be absolute", rawURL)
	}
	if u.User != nil {
		return fmt.Errorf("issuer_url: OAuth issuer URL %q must not include credentials", rawURL)
	}
	if u.Fragment != "" {
		return fmt.Errorf("issuer_url: OAuth issuer URL %q must not include a fragment", rawURL)
	}
	if u.RawQuery != "" {
		return fmt.Errorf("issuer_url: OAuth issuer URL %q must not include a query string", rawURL)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackOAuthHost(u.Hostname()) {
			return nil
		}
		return fmt.Errorf("issuer_url: OAuth issuer URL %q must use https unless the host is localhost or loopback", rawURL)
	default:
		return fmt.Errorf("issuer_url: OAuth issuer URL %q must use http or https", rawURL)
	}
}

// validateOIDCEndpoints checks that every non-empty endpoint URL in cfg uses
// the https scheme, shares the same hostname as issuerURL, and stays within
// the issuer's path scope. This prevents a malicious OIDC server from
// redirecting token traffic to another host or sibling tenant. Validation is
// relaxed when issuerURL itself uses http:// loopback (e.g. local dev), but
// those endpoints must also stay on loopback.
func validateOIDCEndpoints(issuerURL string, cfg *OIDCConfig) error {
	issuer, err := url.Parse(issuerURL)
	if err != nil {
		return fmt.Errorf("OIDC: invalid issuer URL %q: %w", issuerURL, err)
	}
	if issuer.Scheme == "http" && isLoopbackOAuthHost(issuer.Hostname()) {
		for _, endpoint := range []string{cfg.AuthorizationEndpoint, cfg.DeviceAuthorizationEndpoint, cfg.TokenEndpoint} {
			if endpoint == "" {
				continue
			}
			u, err := url.Parse(endpoint)
			if err != nil {
				return fmt.Errorf("OIDC: invalid endpoint URL %q: %w", endpoint, err)
			}
			if u.Scheme != "http" || !isLoopbackOAuthHost(u.Hostname()) {
				return fmt.Errorf("OIDC: loopback issuer endpoint %q must stay on http loopback", endpoint)
			}
		}
		return nil
	}
	if issuer.Scheme != "https" {
		return fmt.Errorf("OIDC: issuer URL %q must use https unless the host is localhost or loopback http", issuerURL)
	}
	for _, endpoint := range []string{cfg.AuthorizationEndpoint, cfg.DeviceAuthorizationEndpoint, cfg.TokenEndpoint} {
		if endpoint == "" {
			continue
		}
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("OIDC: invalid endpoint URL %q: %w", endpoint, err)
		}
		if u.Scheme != "https" {
			return fmt.Errorf("OIDC: endpoint %q must use https", endpoint)
		}
		endpointHost, err := canonicalOIDCHostname(u.Hostname())
		if err != nil {
			return fmt.Errorf("OIDC: endpoint hostname %q is invalid: %w", u.Hostname(), err)
		}
		issuerHost, err := canonicalOIDCHostname(issuer.Hostname())
		if err != nil {
			return fmt.Errorf("OIDC: issuer hostname %q is invalid: %w", issuer.Hostname(), err)
		}
		if endpointHost != issuerHost {
			return fmt.Errorf("OIDC: endpoint hostname %q does not match issuer hostname %q", u.Hostname(), issuer.Hostname())
		}
		if !isPathWithinIssuerScope(issuer.Path, u.Path) {
			return fmt.Errorf("OIDC: endpoint path %q is outside issuer path %q", u.Path, issuer.Path)
		}
	}
	return nil
}

func canonicalOIDCHostname(host string) (string, error) {
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", err
	}
	return strings.ToLower(ascii), nil
}

func isPathWithinIssuerScope(issuerPath, endpointPath string) bool {
	issuerPath = path.Clean("/" + strings.TrimPrefix(issuerPath, "/"))
	endpointPath = path.Clean("/" + strings.TrimPrefix(endpointPath, "/"))

	if issuerPath == "/" {
		return true
	}
	issuerPrefix := strings.TrimSuffix(issuerPath, "/") + "/"
	return endpointPath == issuerPath || strings.HasPrefix(endpointPath, issuerPrefix)
}

func validateDirectOAuthEndpoint(paramName, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%s: invalid OAuth endpoint URL %q: %w", paramName, rawURL, err)
	}
	if !u.IsAbs() || u.Host == "" {
		return fmt.Errorf("%s: OAuth endpoint URL %q must be absolute", paramName, rawURL)
	}
	if u.User != nil {
		return fmt.Errorf("%s: OAuth endpoint URL %q must not include credentials", paramName, rawURL)
	}
	if u.Fragment != "" {
		return fmt.Errorf("%s: OAuth endpoint URL %q must not include a fragment", paramName, rawURL)
	}
	if u.RawQuery != "" {
		return fmt.Errorf("%s: OAuth endpoint URL %q must not include a query string", paramName, rawURL)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackOAuthHost(u.Hostname()) {
			return nil
		}
		return fmt.Errorf("%s: OAuth endpoint URL %q must use https unless the host is localhost or loopback", paramName, rawURL)
	default:
		return fmt.Errorf("%s: OAuth endpoint URL %q must use http or https", paramName, rawURL)
	}
}

func isLoopbackOAuthHost(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func readOAuthEndpointBody(r io.Reader) ([]byte, error) {
	limited := &io.LimitedReader{R: r, N: maxOAuthEndpointBodyBytes + 1}
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(body) > maxOAuthEndpointBodyBytes {
		return nil, fmt.Errorf("OAuth endpoint response exceeds %d bytes", maxOAuthEndpointBodyBytes)
	}
	return body, nil
}

// FetchToken posts a token request to tokenURL and returns a CachedToken.
// Pass nil for client to use http.DefaultClient.
func FetchToken(ctx context.Context, client *http.Client, tokenURL string, form url.Values, params map[string]string) (CachedToken, error) {
	if client == nil {
		client = http.DefaultClient
	}
	client = oauthTokenHTTPClient(client)
	if form == nil {
		form = url.Values{}
	}
	applyOAuthTokenExtraParams(form, params)
	if err := applyTokenAuthHeaders(form, params); err != nil {
		return CachedToken{}, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return CachedToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	applyTokenAuthHeader(req, params)
	resp, err := client.Do(req)
	if err != nil {
		return CachedToken{}, err
	}
	defer resp.Body.Close()
	body, err := readOAuthEndpointBody(resp.Body)
	if err != nil {
		return CachedToken{}, fmt.Errorf("reading token endpoint response: %w", err)
	}
	if resp.StatusCode != 200 {
		return CachedToken{}, parseTokenEndpointError(resp.StatusCode, body)
	}
	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return CachedToken{}, fmt.Errorf("decoding token response: %w", err)
	}
	if strings.TrimSpace(tok.AccessToken) == "" {
		return CachedToken{}, fmt.Errorf("token endpoint response missing access_token")
	}
	if tt := strings.TrimSpace(tok.TokenType); tt != "" && !strings.EqualFold(tt, "bearer") {
		return CachedToken{}, fmt.Errorf("token endpoint response has unsupported token_type %q", tok.TokenType)
	}
	ct := CachedToken{
		AccessToken:  tok.AccessToken,
		TokenType:    tok.TokenType,
		RefreshToken: tok.RefreshToken,
	}
	if tok.ExpiresIn > 0 {
		ct.Expiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	}
	return ct, nil
}

func oauthTokenHTTPClient(src *http.Client) *http.Client {
	if src == nil {
		src = http.DefaultClient
	}
	clone := *src
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}

func parseTokenEndpointError(statusCode int, body []byte) error {
	var decoded struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &decoded)
	return &tokenEndpointError{
		StatusCode:  statusCode,
		ErrorCode:   decoded.Error,
		Description: decoded.ErrorDescription,
		Body:        redactTokenEndpointBody(body),
	}
}

func isTokenEndpointErrorCode(err error, code string) bool {
	var tokenErr *tokenEndpointError
	return errors.As(err, &tokenErr) && tokenErr.ErrorCode == code
}

func redactTokenEndpointBody(body []byte) string {
	var decoded any
	if err := json.Unmarshal(body, &decoded); err == nil {
		redactSecretFields(decoded)
		if marshaled, err := json.Marshal(decoded); err == nil {
			body = marshaled
		}
	}
	const maxLen = 256
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}

func redactSecretFields(v any) {
	switch typed := v.(type) {
	case map[string]any:
		for key, value := range typed {
			if secrets.IsOAuthErrorBodyKey(key) {
				typed[key] = "***"
				continue
			}
			redactSecretFields(value)
		}
	case []any:
		for _, item := range typed {
			redactSecretFields(item)
		}
	}
}

func oauthAuthMethod(params map[string]string) (string, error) {
	switch method := params["auth_method"]; method {
	case "", authMethodClientSecretPost:
		return authMethodClientSecretPost, nil
	case authMethodClientSecretBasic:
		return authMethodClientSecretBasic, nil
	default:
		return "", fmt.Errorf("unsupported auth_method %q", method)
	}
}

func applyTokenAuthHeaders(form url.Values, params map[string]string) error {
	method, err := oauthAuthMethod(params)
	if err != nil {
		return err
	}
	if method == authMethodClientSecretBasic {
		form.Del("client_secret")
		return nil
	}
	clientSecret := params["client_secret"]
	if clientSecret != "" && form.Get("client_secret") == "" {
		form.Set("client_secret", clientSecret)
	}
	return nil
}

func applyTokenAuthHeader(req *http.Request, params map[string]string) {
	method, err := oauthAuthMethod(params)
	if err != nil || method != authMethodClientSecretBasic {
		return
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(url.QueryEscape(params["client_id"]) + ":" + url.QueryEscape(params["client_secret"])))
	req.Header.Set("Authorization", "Basic "+encoded)
}

func applyOAuthTokenExtraParams(form url.Values, params map[string]string) {
	for key, value := range extraOAuthParams(params, map[string]bool{
		"_cache_key":             true,
		"authorize_url":          true,
		"cache_key":              true,
		callbackErrorHTMLParam:   true,
		callbackSuccessHTMLParam: true,
		"issuer_url":             true,
		// TODO(openapi-3.2): use oauth2_metadata_url for RFC 8414 metadata
		// discovery in place of, or alongside, issuer_url.
		"oauth2_metadata_url": true,
		"redirect_cert":       true,
		"redirect_key":        true,
		"redirect_path":       true,
		"redirect_port":       true,
		"redirect_scheme":     true,
		"token_url":           true,
	}) {
		if form.Get(key) == "" {
			form.Set(key, value)
		}
	}
}

func refreshOAuthToken(ctx context.Context, client *http.Client, params map[string]string, tokenURL, refreshToken string) (CachedToken, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {params["client_id"]},
	}
	token, err := FetchToken(ctx, client, tokenURL, form, params)
	if err != nil {
		return CachedToken{}, err
	}
	nextRefreshToken := strings.TrimSpace(token.RefreshToken)
	if nextRefreshToken == "" || nextRefreshToken == refreshToken {
		token.RefreshToken = refreshToken
	} else {
		token.RefreshToken = nextRefreshToken
	}
	return token, nil
}

type refreshTokenStore interface {
	TokenStore
	Refresh(key string, force bool, refresh func(CachedToken) (CachedToken, error)) (*CachedToken, bool, error)
}

func refreshCachedOAuthToken(cache TokenStore, cacheKey string, force bool, refresh func(CachedToken) (CachedToken, error)) (*CachedToken, bool, error) {
	if store, ok := cache.(refreshTokenStore); ok {
		return store.Refresh(cacheKey, force, refresh)
	}
	cached, err := cache.Get(cacheKey)
	if err != nil || cached == nil {
		return cached, false, err
	}
	if !force && !cached.IsExpired() {
		return cached, false, nil
	}
	if cached.RefreshToken == "" {
		return cached, false, nil
	}
	refreshed, err := refresh(*cached)
	if err != nil {
		return cached, false, err
	}
	if err := cache.Set(cacheKey, refreshed); err != nil {
		return nil, false, err
	}
	return &refreshed, true, nil
}

func cachedOAuthAccessToken(cache TokenStore, cacheKey string, force bool, refresh func(CachedToken) (CachedToken, error)) (string, bool, error) {
	if cache == nil || cacheKey == "" {
		return "", false, nil
	}
	cached, err := cache.Get(cacheKey)
	if err != nil || cached == nil {
		return "", false, nil
	}
	if !force && !cached.IsExpired() {
		return cached.AccessToken, true, nil
	}
	if cached.RefreshToken == "" {
		return "", false, nil
	}
	refreshed, didRefresh, err := refreshCachedOAuthToken(cache, cacheKey, force, refresh)
	if err != nil {
		return "", false, err
	}
	if refreshed == nil {
		return "", false, nil
	}
	if didRefresh || (!force && !refreshed.IsExpired()) {
		return refreshed.AccessToken, true, nil
	}
	return "", false, nil
}

func extraOAuthParams(params map[string]string, reserved map[string]bool) map[string]string {
	extra := map[string]string{}
	for key, value := range params {
		if reserved[key] {
			continue
		}
		switch key {
		case "auth_method", "client_id", "client_secret", "scopes":
			continue
		}
		extra[key] = value
	}
	return extra
}
