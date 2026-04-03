package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
