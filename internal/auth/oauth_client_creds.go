package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ClientCredentials implements the OAuth2 client credentials flow (RFC 6749 §4.4).
// The token is cached in Cache under params["_cache_key"]. When the cached
// token is expired the handler fetches a new one.
type ClientCredentials struct {
	// Cache stores fetched tokens. If nil, tokens are not cached.
	Cache *TokenCache
	// HTTPClient is used for token requests. Defaults to http.DefaultClient when nil.
	HTTPClient *http.Client
}

func (h *ClientCredentials) Parameters() []Param {
	return []Param{
		{Name: "client_id", Description: "OAuth2 client ID", Required: true},
		{Name: "client_secret", Description: "OAuth2 client secret", Required: true, Secret: true},
		{Name: "token_url", Description: "OAuth2 token endpoint URL", Required: false},
		{Name: "issuer_url", Description: "OIDC issuer URL (used for discovery when token_url is absent)", Required: false},
		{Name: "scopes", Description: "Space-separated OAuth2 scopes to request", Required: false},
	}
}

func (h *ClientCredentials) OnRequest(req *http.Request, params map[string]string) error {
	token, err := h.resolveToken(req.Context(), params)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (h *ClientCredentials) resolveToken(ctx context.Context, params map[string]string) (string, error) {
	cacheKey := params["_cache_key"]

	// Check cache.
	if h.Cache != nil && cacheKey != "" {
		cached, err := h.Cache.Get(cacheKey)
		if err == nil && cached != nil && !cached.IsExpired() {
			return cached.AccessToken, nil
		}
	}

	// Resolve token URL (possibly via OIDC discovery).
	tokenURL := params["token_url"]
	if tokenURL == "" {
		issuer := params["issuer_url"]
		if issuer == "" {
			return "", fmt.Errorf("oauth-client-credentials: token_url or issuer_url is required")
		}
		oidc, err := DiscoverOIDC(ctx, h.HTTPClient, issuer)
		if err != nil {
			return "", err
		}
		tokenURL = oidc.TokenEndpoint
	}

	// Fetch a new token.
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {params["client_id"]},
		"client_secret": {params["client_secret"]},
	}
	if scopes := params["scopes"]; scopes != "" {
		form.Set("scope", scopes)
	}

	ct, err := FetchToken(ctx, h.HTTPClient, tokenURL, form.Encode())
	if err != nil {
		return "", fmt.Errorf("oauth-client-credentials: %w", err)
	}

	if h.Cache != nil && cacheKey != "" {
		_ = h.Cache.Set(cacheKey, ct)
	}

	return ct.AccessToken, nil
}
