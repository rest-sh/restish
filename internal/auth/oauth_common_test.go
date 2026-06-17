package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"github.com/rest-sh/restish/v2/auth"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOAuthParametersDeclareCommonProviderParams(t *testing.T) {
	for name, params := range map[string][]auth.Param{
		"auth-code":          (&AuthorizationCode{}).Parameters(),
		"device-code":        (&DeviceCode{}).Parameters(),
		"client-credentials": (&ClientCredentials{}).Parameters(),
	} {
		for _, want := range []string{"audience", "resource", "organization"} {
			if !hasAuthParam(params, want) {
				t.Fatalf("%s parameters missing %q: %#v", name, want, params)
			}
		}
	}
	for name, params := range map[string][]auth.Param{
		"auth-code":   (&AuthorizationCode{}).Parameters(),
		"device-code": (&DeviceCode{}).Parameters(),
	} {
		scopes, ok := authParam(params, "scopes")
		if !ok || !strings.Contains(scopes.Description, "offline_access") {
			t.Fatalf("%s scopes help should mention offline_access, got %#v", name, scopes)
		}
	}
}

func TestCachedUsableOAuthAccessTokenDoesNotRefreshExpiredToken(t *testing.T) {
	cache := auth.NewTokenCache(t.TempDir() + "/tokens.cbor")
	if err := cache.Set("svc:default", auth.CachedToken{
		AccessToken:  "old-token",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	token, ok, err := cachedUsableOAuthAccessToken(cache, "svc:default")
	if err != nil {
		t.Fatalf("cached usable token: %v", err)
	}
	if ok || token != "" {
		t.Fatalf("cached usable token = %q, %v; want no token", token, ok)
	}
}

func TestClearRejectedOAuthTokenDeletesCacheEntry(t *testing.T) {
	cache := auth.NewTokenCache(t.TempDir() + "/tokens.cbor")
	if err := cache.Set("svc:default", auth.CachedToken{AccessToken: "old", RefreshToken: "bad"}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	var stderr bytes.Buffer
	clearRejectedOAuthToken(cache, "svc:default", &stderr)
	if got, err := cache.Get("svc:default"); err != nil {
		t.Fatalf("get cache: %v", err)
	} else if got != nil {
		t.Fatalf("cache entry still present: %#v", got)
	}
	if !strings.Contains(stderr.String(), "cleared cached token") {
		t.Fatalf("expected clear warning, got %q", stderr.String())
	}
}

func TestWarnIfMissingOAuthRefreshToken(t *testing.T) {
	var stderr bytes.Buffer
	warnIfMissingOAuthRefreshToken(&stderr, map[string]string{"_cache_key": "svc:default"}, auth.CachedToken{AccessToken: "token"})
	if !strings.Contains(stderr.String(), "offline_access") {
		t.Fatalf("expected offline_access warning, got %q", stderr.String())
	}
}

func hasAuthParam(params []auth.Param, name string) bool {
	_, ok := authParam(params, name)
	return ok
}

func authParam(params []auth.Param, name string) (auth.Param, bool) {
	for _, param := range params {
		if param.Name == name {
			return param, true
		}
	}
	return auth.Param{}, false
}

func TestValidateOIDCEndpoints(t *testing.T) {
	cases := []struct {
		name      string
		issuer    string
		authURL   string
		tokenURL  string
		wantError bool
	}{
		{
			name:      "valid same hostname",
			issuer:    "https://auth.example.com",
			authURL:   "https://auth.example.com/authorize",
			tokenURL:  "https://auth.example.com/token",
			wantError: false,
		},
		{
			name:      "attacker token endpoint",
			issuer:    "https://auth.example.com",
			authURL:   "https://auth.example.com/authorize",
			tokenURL:  "https://attacker.com/steal",
			wantError: true,
		},
		{
			name:      "http token endpoint with https issuer",
			issuer:    "https://auth.example.com",
			tokenURL:  "http://auth.example.com/token",
			wantError: true,
		},
		{
			name:      "loopback http issuer accepts loopback endpoints",
			issuer:    "http://localhost:8080",
			authURL:   "http://localhost:8080/authorize",
			tokenURL:  "http://localhost:8080/token",
			wantError: false,
		},
		{
			name:      "public http issuer rejected",
			issuer:    "http://auth.example.com",
			authURL:   "http://auth.example.com/authorize",
			tokenURL:  "http://auth.example.com/token",
			wantError: true,
		},
		{
			name:      "ftp issuer rejected",
			issuer:    "ftp://auth.example.com",
			tokenURL:  "ftp://auth.example.com/token",
			wantError: true,
		},
		{
			name:      "custom issuer scheme rejected",
			issuer:    "custom://auth.example.com",
			tokenURL:  "custom://auth.example.com/token",
			wantError: true,
		},
		{
			name:      "loopback http issuer rejects remote token endpoint",
			issuer:    "http://localhost:8080",
			tokenURL:  "https://auth.example.com/token",
			wantError: true,
		},
		{
			name:      "empty endpoints are allowed",
			issuer:    "https://auth.example.com",
			authURL:   "",
			tokenURL:  "",
			wantError: false,
		},
		{
			name:      "case-insensitive hostname match",
			issuer:    "https://Auth.Example.COM",
			tokenURL:  "https://auth.example.com/token",
			wantError: false,
		},
		{
			name:      "idna hostname match",
			issuer:    "https://bücher.example",
			tokenURL:  "https://xn--bcher-kva.example/token",
			wantError: false,
		},
		{
			name:      "path-scoped issuer allows child endpoint paths",
			issuer:    "https://auth.example.com/realms/demo",
			authURL:   "https://auth.example.com/realms/demo/protocol/openid-connect/auth",
			tokenURL:  "https://auth.example.com/realms/demo/protocol/openid-connect/token",
			wantError: false,
		},
		{
			name:      "path-scoped issuer rejects sibling tenant endpoint paths",
			issuer:    "https://auth.example.com/realms/demo",
			tokenURL:  "https://auth.example.com/realms/other/protocol/openid-connect/token",
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &OIDCConfig{
				AuthorizationEndpoint: tc.authURL,
				TokenEndpoint:         tc.tokenURL,
			}
			err := validateOIDCEndpoints(tc.issuer, cfg)
			if tc.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestOAuthTokenHTTPClientPreservesTimeoutAndBlocksRedirects(t *testing.T) {
	src := &http.Client{Timeout: 3}
	got := oauthTokenHTTPClient(src)
	if got == src {
		t.Fatal("expected cloned client")
	}
	if got.Timeout != src.Timeout {
		t.Fatalf("Timeout = %v, want %v", got.Timeout, src.Timeout)
	}
	err := got.CheckRedirect(&http.Request{}, nil)
	if !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect = %v, want ErrUseLastResponse", err)
	}
}

func TestParseTokenEndpointErrorRedactsSecrets(t *testing.T) {
	err := parseTokenEndpointError(400, []byte(`{
		"error":"invalid_client",
		"token_type":"bearer",
		"client_secret":"top-secret",
		"refresh_token":"refresh-secret",
		"details":{"access_token":"abc"}
	}`))
	var tokenErr *tokenEndpointError
	if !strings.Contains(err.Error(), "invalid_client") {
		t.Fatalf("expected error code in message, got %v", err)
	}
	if !errorAsToken(err, &tokenErr) {
		t.Fatalf("expected token endpoint error, got %T", err)
	}
	for _, secret := range []string{"top-secret", "refresh-secret", "abc"} {
		if strings.Contains(tokenErr.Body, secret) {
			t.Fatalf("expected body redaction, got %q", tokenErr.Body)
		}
	}
	if !strings.Contains(tokenErr.Body, `"token_type":"bearer"`) {
		t.Fatalf("token_type should not be redacted, got %q", tokenErr.Body)
	}
}

func TestApplyTokenAuthHeaderPercentEncodesBasicCredentials(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://auth.example.com/token", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	params := map[string]string{
		"client_id":     "id:with+space ü",
		"client_secret": "sec:ret+space ü",
		"auth_method":   authMethodClientSecretBasic,
	}
	applyTokenAuthHeader(req, params)

	const prefix = "Basic "
	got := req.Header.Get("Authorization")
	if !strings.HasPrefix(got, prefix) {
		t.Fatalf("Authorization = %q, want Basic header", got)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, prefix))
	if err != nil {
		t.Fatalf("decode Authorization: %v", err)
	}
	want := url.QueryEscape(params["client_id"]) + ":" + url.QueryEscape(params["client_secret"])
	if string(decoded) != want {
		t.Fatalf("decoded credentials = %q, want %q", decoded, want)
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		t.Fatalf("decoded credentials should contain one delimiter: %q", decoded)
	}
	id, err := url.QueryUnescape(parts[0])
	if err != nil {
		t.Fatalf("unescape client_id: %v", err)
	}
	secret, err := url.QueryUnescape(parts[1])
	if err != nil {
		t.Fatalf("unescape client_secret: %v", err)
	}
	if got := id; got != params["client_id"] {
		t.Fatalf("client_id round trip = %q, want %q", got, params["client_id"])
	}
	if got := secret; got != params["client_secret"] {
		t.Fatalf("client_secret round trip = %q, want %q", got, params["client_secret"])
	}
}

func TestApplyOAuthTokenExtraParamsOmitsMetadataURL(t *testing.T) {
	form := url.Values{}
	applyOAuthTokenExtraParams(form, map[string]string{
		"audience":               "https://api.example.com/",
		callbackErrorHTMLParam:   "<html>error</html>",
		callbackSuccessHTMLParam: "<html>success</html>",
		"oauth2_metadata_url":    "https://auth.example.com/.well-known/oauth-authorization-server",
	})
	if got := form.Get("audience"); got != "https://api.example.com/" {
		t.Fatalf("audience = %q", got)
	}
	if got := form.Get(callbackSuccessHTMLParam); got != "" {
		t.Fatalf("%s should not be forwarded, got %q", callbackSuccessHTMLParam, got)
	}
	if got := form.Get(callbackErrorHTMLParam); got != "" {
		t.Fatalf("%s should not be forwarded, got %q", callbackErrorHTMLParam, got)
	}
	if got := form.Get("oauth2_metadata_url"); got != "" {
		t.Fatalf("oauth2_metadata_url should not be forwarded, got %q", got)
	}
}

func TestValidateDirectOAuthEndpoint(t *testing.T) {
	cases := []struct {
		name      string
		param     string
		rawURL    string
		wantError bool
	}{
		{name: "valid https", param: "token_url", rawURL: "https://auth.example.com/token"},
		{name: "localhost http", param: "token_url", rawURL: "http://localhost:8080/token"},
		{name: "ipv4 loopback http", param: "token_url", rawURL: "http://127.0.0.1:8080/token"},
		{name: "ipv6 loopback http", param: "token_url", rawURL: "http://[::1]:8080/token"},
		{name: "public http rejected", param: "token_url", rawURL: "http://auth.example.com/token", wantError: true},
		{name: "credentials rejected", param: "token_url", rawURL: "https://user:pass@auth.example.com/token", wantError: true},
		{name: "fragment rejected", param: "authorize_url", rawURL: "https://auth.example.com/authorize#frag", wantError: true},
		{name: "query rejected", param: "authorize_url", rawURL: "https://auth.example.com/authorize?prompt=login", wantError: true},
		{name: "relative rejected", param: "token_url", rawURL: "/token", wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDirectOAuthEndpoint(tc.param, tc.rawURL)
			if tc.wantError && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveOAuthEndpoint(t *testing.T) {
	cases := []struct {
		name      string
		rawURL    string
		baseURL   string
		want      string
		wantError string
	}{
		{
			name:    "absolute https",
			rawURL:  "https://auth.example.com/token",
			baseURL: "https://api.example.com/v1",
			want:    "https://auth.example.com/token",
		},
		{
			name:    "path relative resolves under base path",
			rawURL:  "oauth2/token",
			baseURL: "https://api.example.com/v1",
			want:    "https://api.example.com/v1/oauth2/token",
		},
		{
			name:    "root relative resolves at host root",
			rawURL:  "/oauth2/token",
			baseURL: "https://api.example.com/v1",
			want:    "https://api.example.com/oauth2/token",
		},
		{
			name:      "scheme relative rejected",
			rawURL:    "//auth.example.com/token",
			baseURL:   "https://api.example.com/v1",
			wantError: "scheme-relative",
		},
		{
			name:      "relative without base rejected",
			rawURL:    "oauth2/token",
			wantError: "requires API base_url",
		},
		{
			name:      "relative query rejected after resolution",
			rawURL:    "oauth2/token?audience=api",
			baseURL:   "https://api.example.com/v1",
			wantError: "query string",
		},
		{
			name:      "relative public http still rejected",
			rawURL:    "oauth2/token",
			baseURL:   "http://api.example.com/v1",
			wantError: "must use https",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveOAuthEndpoint("token_url", tc.rawURL, tc.baseURL)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want containing %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("resolved = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateOAuthIssuerURL(t *testing.T) {
	cases := []struct {
		name      string
		rawURL    string
		wantError bool
	}{
		{name: "valid https", rawURL: "https://auth.example.com/realms/demo"},
		{name: "localhost http", rawURL: "http://localhost:8080"},
		{name: "loopback http", rawURL: "http://127.0.0.1:8080"},
		{name: "public http rejected", rawURL: "http://auth.example.com", wantError: true},
		{name: "userinfo rejected", rawURL: "https://user:pass@auth.example.com", wantError: true},
		{name: "query rejected", rawURL: "https://auth.example.com?tenant=a", wantError: true},
		{name: "fragment rejected", rawURL: "https://auth.example.com#frag", wantError: true},
		{name: "relative rejected", rawURL: "/issuer", wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOAuthIssuerURL(tc.rawURL)
			if tc.wantError && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDiscoverOIDCRejectsOversizedBody(t *testing.T) {
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		return testResponse(http.StatusOK, "application/json", strings.Repeat("x", maxOAuthEndpointBodyBytes+1)), nil
	})

	_, err := DiscoverOIDC(context.Background(), client, "https://auth.example.com")
	if err == nil {
		t.Fatal("expected oversized discovery body error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestFetchTokenRejectsOversizedBody(t *testing.T) {
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		return testResponse(http.StatusOK, "application/json", strings.Repeat("x", maxOAuthEndpointBodyBytes+1)), nil
	})

	_, err := FetchToken(context.Background(), client, "https://auth.example.com/token", url.Values{}, nil)
	if err == nil {
		t.Fatal("expected oversized body error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size limit error, got %v", err)
	}
}

func TestFetchTokenAcceptsNumericStringExpiresInAndSendsAccept(t *testing.T) {
	var gotAccept string
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		gotAccept = r.Header.Get("Accept")
		return testResponse(http.StatusOK, "application/json", `{"access_token":"abc","token_type":"bearer","expires_in":"3600"}`), nil
	})

	token, err := FetchToken(context.Background(), client, "https://auth.example.com/token", url.Values{}, nil)
	if err != nil {
		t.Fatalf("FetchToken: %v", err)
	}
	if token.AccessToken != "abc" {
		t.Fatalf("AccessToken = %q, want abc", token.AccessToken)
	}
	if token.Expiry.IsZero() {
		t.Fatal("expected expiry from numeric string expires_in")
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q, want application/json", gotAccept)
	}
}

func TestFetchTokenValidatesAccessTokenAndTokenType(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "empty object", body: `{}`, want: "access_token"},
		{name: "empty access token", body: `{"access_token":"","token_type":"bearer"}`, want: "access_token"},
		{name: "unsupported token type", body: `{"access_token":"abc","token_type":"mac"}`, want: "token_type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
				return testResponse(http.StatusOK, "application/json", tc.body), nil
			})

			_, err := FetchToken(context.Background(), client, "https://auth.example.com/token", url.Values{}, nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q in error, got %v", tc.want, err)
			}
		})
	}
}

func TestFetchTokenAcceptsAbsentAndCaseInsensitiveBearerTokenType(t *testing.T) {
	for _, body := range []string{
		`{"access_token":"abc"}`,
		`{"access_token":"abc","token_type":"Bearer"}`,
	} {
		t.Run(body, func(t *testing.T) {
			client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
				return testResponse(http.StatusOK, "application/json", body), nil
			})
			token, err := FetchToken(context.Background(), client, "https://auth.example.com/token", url.Values{}, nil)
			if err != nil {
				t.Fatalf("FetchToken: %v", err)
			}
			if token.AccessToken != "abc" {
				t.Fatalf("AccessToken = %q, want abc", token.AccessToken)
			}
		})
	}
}

func TestFetchTokenDoesNotFollowRedirects(t *testing.T) {
	calls := 0
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			t.Fatalf("unexpected redirected token request to %s", r.URL)
		}
		resp := testResponse(http.StatusTemporaryRedirect, "text/plain", "redirect")
		resp.Header.Set("Location", "https://attacker.example/token")
		return resp, nil
	})

	_, err := FetchToken(context.Background(), client, "https://auth.example.com/token", url.Values{"client_secret": {"secret"}}, nil)
	if err == nil {
		t.Fatal("expected token endpoint status error")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestFetchTokenRejectsInvalidStringExpiresIn(t *testing.T) {
	client := testHTTPClient(func(r *http.Request) (*http.Response, error) {
		return testResponse(http.StatusOK, "application/json", `{"access_token":"abc","token_type":"bearer","expires_in":"soon"}`), nil
	})

	_, err := FetchToken(context.Background(), client, "https://auth.example.com/token", url.Values{}, nil)
	if err == nil {
		t.Fatal("expected invalid expires_in error")
	}
	if !strings.Contains(err.Error(), "expires_in") {
		t.Fatalf("expected expires_in error, got %v", err)
	}
}

func errorAsToken(err error, target **tokenEndpointError) bool {
	return errors.As(err, target)
}
