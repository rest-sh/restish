package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const deviceCodeGrantType = "urn:ietf:params:oauth:grant-type:device_code"

type deviceAuthorizationResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
	Message                 string `json:"message,omitempty"`
}

// DeviceCode implements OAuth 2.0 Device Authorization Grant (RFC 8628).
type DeviceCode struct {
	Cache      TokenStore
	HTTPClient *http.Client
	Stderr     io.Writer
}

func (h *DeviceCode) Parameters() []Param {
	return appendOAuthPassthroughParams([]Param{
		{Name: "client_id", Description: "OAuth2 client ID", Required: true},
		{Name: "client_secret", Description: "OAuth2 client secret (optional)", Required: false, Secret: true},
		{Name: "auth_method", Description: "OAuth2 client auth method: client_secret_post (default) or client_secret_basic", Required: false},
		{Name: "device_authorization_url", Description: "OAuth2 device authorization endpoint URL", Required: false},
		{Name: "token_url", Description: "OAuth2 token endpoint URL", Required: false},
		{Name: "issuer_url", Description: "OIDC issuer URL (used for discovery when endpoints are absent)", Required: false},
		{Name: "scopes", Description: "Space-separated OAuth2 scopes to request; some providers require offline_access for refresh tokens", Required: false},
	})
}

func (h *DeviceCode) OnRequest(req *http.Request, params map[string]string) error {
	return h.authenticateRequest(req, params, false)
}

func (h *DeviceCode) authenticateRequest(req *http.Request, params map[string]string, force bool) error {
	token, err := h.resolveToken(req.Context(), params, force)
	if err != nil {
		return err
	}
	bearerAuth(req, token)
	return nil
}

func (h *DeviceCode) resolveToken(ctx context.Context, params map[string]string, force bool) (string, error) {
	cacheKey := params["_cache_key"]
	var tokenURL string
	var tokenURLErr error
	accessToken, ok, err := cachedOAuthAccessToken(h.Cache, cacheKey, force, func(cached CachedToken) (CachedToken, error) {
		if tokenURL == "" && tokenURLErr == nil {
			tokenURL, tokenURLErr = h.resolveTokenURL(ctx, params)
		}
		if tokenURLErr != nil {
			return CachedToken{}, tokenURLErr
		}
		return h.doRefresh(ctx, params, tokenURL, cached.RefreshToken)
	})
	if ok {
		return accessToken, nil
	}
	if err != nil {
		if h.Stderr != nil {
			fmt.Fprintf(h.Stderr, "OAuth refresh failed: %v\n", err)
		}
		if !isTokenEndpointErrorCode(err, "invalid_grant") {
			return "", err
		}
		clearRejectedOAuthToken(h.Cache, cacheKey, h.Stderr)
	}

	deviceURL, tokenURL, err := h.resolveEndpoints(ctx, params)
	if err != nil {
		return "", err
	}
	token, err := h.runFlow(ctx, params, deviceURL, tokenURL)
	if err != nil {
		return "", err
	}
	if h.Cache != nil && cacheKey != "" {
		_ = h.Cache.Set(cacheKey, token)
	}
	warnIfMissingOAuthRefreshToken(h.Stderr, params, token)
	return token.AccessToken, nil
}

func (h *DeviceCode) doRefresh(ctx context.Context, params map[string]string, tokenURL, refreshToken string) (CachedToken, error) {
	return refreshOAuthToken(ctx, h.HTTPClient, params, tokenURL, refreshToken)
}

func (h *DeviceCode) resolveTokenURL(ctx context.Context, params map[string]string) (string, error) {
	if tokenURL := params["token_url"]; tokenURL != "" {
		resolved, err := resolveOAuthEndpoint("token_url", tokenURL, params["_base_url"])
		if err != nil {
			return "", err
		}
		return resolved, nil
	}
	issuer := params["issuer_url"]
	if issuer == "" {
		return "", fmt.Errorf("oauth-device-code: token_url or issuer_url is required for token refresh")
	}
	oidc, err := DiscoverOIDC(ctx, h.HTTPClient, issuer)
	if err != nil {
		return "", err
	}
	if err := validateOIDCEndpoints(issuer, oidc); err != nil {
		return "", err
	}
	if oidc.TokenEndpoint == "" {
		return "", fmt.Errorf("oauth-device-code: issuer discovery did not provide token_endpoint")
	}
	return oidc.TokenEndpoint, nil
}

func (h *DeviceCode) resolveEndpoints(ctx context.Context, params map[string]string) (string, string, error) {
	deviceURL := params["device_authorization_url"]
	tokenURL := params["token_url"]
	if deviceURL != "" {
		resolved, err := resolveOAuthEndpoint("device_authorization_url", deviceURL, params["_base_url"])
		if err != nil {
			return "", "", err
		}
		deviceURL = resolved
	}
	if tokenURL != "" {
		resolved, err := resolveOAuthEndpoint("token_url", tokenURL, params["_base_url"])
		if err != nil {
			return "", "", err
		}
		tokenURL = resolved
	}
	if deviceURL != "" && tokenURL != "" {
		return deviceURL, tokenURL, nil
	}
	issuer := params["issuer_url"]
	if issuer == "" {
		return "", "", fmt.Errorf("oauth-device-code: (device_authorization_url and token_url) or issuer_url is required")
	}
	oidc, err := DiscoverOIDC(ctx, h.HTTPClient, issuer)
	if err != nil {
		return "", "", err
	}
	if err := validateOIDCEndpoints(issuer, oidc); err != nil {
		return "", "", err
	}
	if deviceURL == "" {
		deviceURL = oidc.DeviceAuthorizationEndpoint
	}
	if tokenURL == "" {
		tokenURL = oidc.TokenEndpoint
	}
	if deviceURL == "" || tokenURL == "" {
		return "", "", fmt.Errorf("oauth-device-code: issuer discovery did not provide both device_authorization_endpoint and token_endpoint")
	}
	return deviceURL, tokenURL, nil
}

func (h *DeviceCode) runFlow(ctx context.Context, params map[string]string, deviceURL, tokenURL string) (CachedToken, error) {
	deviceAuth, err := h.requestDeviceAuthorization(ctx, params, deviceURL)
	if err != nil {
		return CachedToken{}, err
	}

	if h.Stderr != nil {
		if deviceAuth.Message != "" {
			fmt.Fprintf(h.Stderr, "%s\n", deviceAuth.Message)
		} else if deviceAuth.VerificationURIComplete != "" {
			fmt.Fprintf(h.Stderr, "Open this URL to authenticate:\n  %s\n", deviceAuth.VerificationURIComplete)
		} else {
			fmt.Fprintf(h.Stderr, "Open %s and enter code %s\n", deviceAuth.VerificationURI, deviceAuth.UserCode)
		}
		if deviceAuth.VerificationURIComplete == "" && deviceAuth.UserCode != "" {
			fmt.Fprintf(h.Stderr, "User code: %s\n", deviceAuth.UserCode)
		}
	}

	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	timeout := authTimeout
	if deviceAuth.ExpiresIn > 0 {
		timeout = time.Duration(deviceAuth.ExpiresIn) * time.Second
	}
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	form := url.Values{
		"grant_type":  {deviceCodeGrantType},
		"device_code": {deviceAuth.DeviceCode},
		"client_id":   {params["client_id"]},
	}
	for {
		token, err := FetchToken(pollCtx, h.HTTPClient, tokenURL, form, params)
		if err == nil {
			return token, nil
		}
		var tokenErr *tokenEndpointError
		if !errors.As(err, &tokenErr) {
			return CachedToken{}, err
		}
		switch tokenErr.ErrorCode {
		case "authorization_pending":
		case "slow_down":
			interval = capDevicePollInterval(interval + 5*time.Second)
		default:
			return CachedToken{}, err
		}

		timer := time.NewTimer(interval)
		select {
		case <-pollCtx.Done():
			timer.Stop()
			if errors.Is(pollCtx.Err(), context.Canceled) {
				return CachedToken{}, pollCtx.Err()
			}
			return CachedToken{}, fmt.Errorf("timed out waiting for device authorization")
		case <-timer.C:
		}
	}
}

func capDevicePollInterval(interval time.Duration) time.Duration {
	const maxDevicePollInterval = 30 * time.Second
	if interval > maxDevicePollInterval {
		return maxDevicePollInterval
	}
	return interval
}

func (h *DeviceCode) requestDeviceAuthorization(ctx context.Context, params map[string]string, deviceURL string) (*deviceAuthorizationResponse, error) {
	form := url.Values{
		"client_id": {params["client_id"]},
	}
	if scopes := params["scopes"]; scopes != "" {
		form.Set("scope", scopes)
	}
	for key, value := range extraOAuthParams(params, map[string]bool{
		"_cache_key":               true,
		"_base_url":                true,
		"authorize_url":            true,
		"cache_key":                true,
		callbackErrorHTMLParam:     true,
		callbackSuccessHTMLParam:   true,
		"device_authorization_url": true,
		"issuer_url":               true,
		"redirect_cert":            true,
		"redirect_key":             true,
		"redirect_path":            true,
		"redirect_port":            true,
		"redirect_scheme":          true,
		"token_url":                true,
	}) {
		if form.Get(key) == "" {
			form.Set(key, value)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := h.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readOAuthEndpointBody(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading device authorization response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseTokenEndpointError(resp.StatusCode, body)
	}

	var decoded deviceAuthorizationResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decoding device authorization response: %w", err)
	}
	return &decoded, nil
}

func (h *DeviceCode) httpClient() *http.Client {
	if h.HTTPClient != nil {
		return h.HTTPClient
	}
	return http.DefaultClient
}

func (h *DeviceCode) Authenticate(ctx context.Context, req *http.Request, ac AuthContext) error {
	h2 := &DeviceCode{
		Cache:      h.Cache,
		HTTPClient: h.HTTPClient,
		Stderr:     h.Stderr,
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
	req = requestWithContext(req, ctx)
	return h2.authenticateRequest(req, authParams(ac), ac.Force)
}

func (h *DeviceCode) SupportsForce() {}
