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
	"strings"
	"time"

	"golang.org/x/net/idna"
)

const (
	authMethodClientSecretPost  = "client_secret_post"
	authMethodClientSecretBasic = "client_secret_basic"
	maxOAuthEndpointBodyBytes   = 1 << 20
)

// tokenResponse is the JSON response from an OAuth2 token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // seconds; 0 means no expiry info
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
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
	var cfg OIDCConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("OIDC discovery from %s: %w", issuerURL, err)
	}
	return &cfg, nil
}

// validateOIDCEndpoints checks that every non-empty endpoint URL in cfg uses
// the https scheme, shares the same hostname as issuerURL, and stays within
// the issuer's path scope. This prevents a malicious OIDC server from
// redirecting token traffic to another host or sibling tenant. Validation is
// skipped when issuerURL itself uses http:// (e.g. local dev).
func validateOIDCEndpoints(issuerURL string, cfg *OIDCConfig) error {
	issuer, err := url.Parse(issuerURL)
	if err != nil {
		return fmt.Errorf("OIDC: invalid issuer URL %q: %w", issuerURL, err)
	}
	// Only enforce for HTTPS issuers; HTTP issuers are already insecure (dev/test).
	if issuer.Scheme != "https" {
		return nil
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
			lower := strings.ToLower(key)
			if strings.Contains(lower, "secret") ||
				strings.Contains(lower, "password") ||
				strings.Contains(lower, "assertion") ||
				strings.Contains(lower, "token") {
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
	req.Header.Set("Authorization", "Basic "+basicAuthValue(params["client_id"], params["client_secret"]))
}

func basicAuthValue(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}

func applyOAuthTokenExtraParams(form url.Values, params map[string]string) {
	for key, value := range extraOAuthParams(params, map[string]bool{
		"_cache_key":    true,
		"authorize_url": true,
		"issuer_url":    true,
		"redirect_port": true,
		"token_url":     true,
	}) {
		if form.Get(key) == "" {
			form.Set(key, value)
		}
	}
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
