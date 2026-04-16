package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
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
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
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
	for _, endpoint := range []string{cfg.AuthorizationEndpoint, cfg.TokenEndpoint} {
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
		if !strings.EqualFold(u.Hostname(), issuer.Hostname()) {
			return fmt.Errorf("OIDC: endpoint hostname %q does not match issuer hostname %q", u.Hostname(), issuer.Hostname())
		}
		if !isPathWithinIssuerScope(issuer.Path, u.Path) {
			return fmt.Errorf("OIDC: endpoint path %q is outside issuer path %q", u.Path, issuer.Path)
		}
	}
	return nil
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

// FetchToken posts formBody (URL-encoded) to tokenURL and returns a CachedToken.
// Pass nil for client to use http.DefaultClient.
func FetchToken(ctx context.Context, client *http.Client, tokenURL, formBody string) (CachedToken, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(formBody))
	if err != nil {
		return CachedToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return CachedToken{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return CachedToken{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
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
