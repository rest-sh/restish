package cli

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	authpkg "github.com/rest-sh/restish/v2/auth"
	"github.com/rest-sh/restish/v2/internal/config"
	"github.com/rest-sh/restish/v2/internal/hypermedia"
	"github.com/rest-sh/restish/v2/internal/request"
)

type testAuthHandler struct{}

func (testAuthHandler) Parameters() []authpkg.Param { return nil }

func (testAuthHandler) OnRequest(req *http.Request, params map[string]string) error {
	req.Header.Set("Authorization", "Bearer "+params["token"])
	return nil
}

func (h testAuthHandler) Authenticate(_ context.Context, req *http.Request, ac authpkg.AuthContext) error {
	return h.OnRequest(req, ac.Params)
}

func TestPrepareRequestBuildsSharedRequestState(t *testing.T) {
	c := New()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"svc": {
				BaseURL: "https://api.example.com",
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Headers: []string{"X-Profile: yes"},
						Query:   []string{"from=profile"},
					},
				},
			},
		},
	}

	prepared, err := c.prepareRequest(
		"svc/items",
		"default",
		request.Options{
			ContentType: "json",
			Headers:     []string{"X-Flag: 1"},
			Query:       []string{"flag=1"},
		},
		map[string]any{"name": "Alice"},
		[]string{"X-Extra: 2"},
		false,
		authHandlerOptions{},
	)
	if err != nil {
		t.Fatalf("prepareRequest() error = %v", err)
	}

	if got, want := prepared.rawURL, "https://api.example.com/items"; got != want {
		t.Fatalf("rawURL = %q, want %q", got, want)
	}
	if got, want := prepared.apiName, "svc"; got != want {
		t.Fatalf("apiName = %q, want %q", got, want)
	}
	if prepared.opts.Transport == nil {
		t.Fatal("expected transport to be pre-built")
	}
	if got, want := strings.Join(prepared.opts.Query, ","), "from=profile,flag=1"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}

	headers := strings.Join(prepared.opts.Headers, "\n")
	for _, want := range []string{
		"X-Profile: yes",
		"X-Flag: 1",
		"X-Extra: 2",
		"Content-Type: application/json",
	} {
		if !strings.Contains(headers, want) {
			t.Fatalf("headers missing %q:\n%s", want, headers)
		}
	}

	body, err := io.ReadAll(prepared.body)
	if err != nil {
		t.Fatalf("ReadAll(body) error = %v", err)
	}
	if got, want := string(body), `{"name":"Alice"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestPrepareRequestNoAuthStripsCredentials(t *testing.T) {
	c := New()
	c.AddAuthHandler("test", testAuthHandler{})
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"svc": {
				BaseURL: "https://api.example.com",
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Headers: []string{
							"Authorization: stale",
							"Cookie: session=abc",
							"X-Profile: kept",
						},
						Auth: &config.AuthConfig{
							Type: "test",
							Params: map[string]string{
								"token": "secret",
							},
						},
					},
				},
			},
		},
	}

	prepared, err := c.prepareRequest("svc/items", "default", request.Options{}, nil, nil, true, authHandlerOptions{})
	if err != nil {
		t.Fatalf("prepareRequest() error = %v", err)
	}

	headers := strings.Join(prepared.opts.Headers, "\n")
	if strings.Contains(strings.ToLower(headers), "authorization:") {
		t.Fatalf("authorization header should have been stripped:\n%s", headers)
	}
	if strings.Contains(strings.ToLower(headers), "cookie:") {
		t.Fatalf("cookie header should have been stripped:\n%s", headers)
	}
	if !strings.Contains(headers, "X-Profile: kept") {
		t.Fatalf("expected non-sensitive header to remain:\n%s", headers)
	}

	req, err := http.NewRequest(http.MethodGet, prepared.rawURL, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if prepared.opts.OnRequest != nil {
		if err := prepared.opts.OnRequest(req); err != nil {
			t.Fatalf("OnRequest() error = %v", err)
		}
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Fatalf("Authorization header = %q, want empty", got)
	}
}

func TestNormalizeHTTPResponseParsesLinks(t *testing.T) {
	c := New()
	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/items", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	httpResp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"Link":         []string{`<https://api.example.com/items?page=2>; rel="next"`},
		},
		Body:    io.NopCloser(strings.NewReader(`{"items":[1]}`)),
		Request: req,
	}

	resp, err := c.normalizeHTTPResponse(httpResp, 0)
	if err != nil {
		t.Fatalf("normalizeHTTPResponse() error = %v", err)
	}
	if got, want := resp.Links["next"], "https://api.example.com/items?page=2"; got != want {
		t.Fatalf("resp.Links[next] = %v, want %q", got, want)
	}
}

type countingBodyLinkParser struct {
	calls int
}

func (p *countingBodyLinkParser) ParseLinks(_ *url.URL, _ http.Header, body any) []hypermedia.Link {
	p.calls++
	if body == nil {
		return nil
	}
	return []hypermedia.Link{{Rel: "self", URI: "https://api.example.com/items/1"}}
}

func TestNormalizeHTTPResponseDefersBodyLinkParsers(t *testing.T) {
	parser := &countingBodyLinkParser{}
	c := New()
	c.linkParsers = []hypermedia.Parser{hypermedia.LinkHeaderParser{}, parser}
	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/items", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	httpResp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"_links":{"self":{"href":"/items/1"}}}`)),
		Request:    req,
	}

	resp, err := c.normalizeHTTPResponse(httpResp, 0)
	if err != nil {
		t.Fatalf("normalizeHTTPResponse() error = %v", err)
	}
	if parser.calls != 0 {
		t.Fatalf("body parser called during normalize: %d", parser.calls)
	}
	c.ensureBodyLinks(resp)
	if parser.calls != 1 {
		t.Fatalf("body parser calls = %d, want 1", parser.calls)
	}
	if got := resp.Links["self"]; got != "https://api.example.com/items/1" {
		t.Fatalf("resp.Links[self] = %v", got)
	}
}

func TestPrepareRequestMissingProfileReturnsError(t *testing.T) {
	c := New()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"svc": {
				BaseURL: "https://api.example.com",
				Profiles: map[string]*config.ProfileConfig{
					"default": {},
				},
			},
		},
	}

	_, err := c.prepareRequest("svc/items", "missing", request.Options{}, nil, nil, false, authHandlerOptions{})
	if err == nil {
		t.Fatal("expected missing profile to return an error")
	}
	if !strings.Contains(err.Error(), `profile "missing" not found for API "svc"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFollowCrossesFirstPartyHost(t *testing.T) {
	if !followCrossesFirstPartyHost("https://api.example.com", "https://redirect.example.com/follow") {
		t.Fatal("expected different host to be treated as cross-host")
	}
	if followCrossesFirstPartyHost("https://api.example.com", "https://api.example.com/follow") {
		t.Fatal("expected same host to stay first-party")
	}
	if !followCrossesFirstPartyHost("https://origin.example.com", "https://redirect.example.com/follow") {
		t.Fatal("expected follow host comparison to use the original first-party host")
	}
	if !followCrossesFirstPartyHost("https://api.example.com:8443", "https://api.example.com/follow") {
		t.Fatal("expected same hostname with a different effective port to be cross-origin")
	}
}

func TestApplyAPIProfilePrefersLongestOperationBasePrefix(t *testing.T) {
	c := New()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"short": {
				BaseURL:       "https://api.example.com/root",
				OperationBase: "/v1",
				Profiles: map[string]*config.ProfileConfig{
					"default": {Headers: []string{"X-API: short"}},
				},
			},
			"long": {
				BaseURL:       "https://api.example.com/root",
				OperationBase: "/v1/admin",
				Profiles: map[string]*config.ProfileConfig{
					"default": {Headers: []string{"X-API: long"}},
				},
			},
		},
	}

	rawURL, apiName, opts, err := c.applyAPIProfile("https://api.example.com/v1/admin/users", "default", request.Options{}, authHandlerOptions{})
	if err != nil {
		t.Fatalf("applyAPIProfile() error = %v", err)
	}
	if rawURL != "https://api.example.com/v1/admin/users" {
		t.Fatalf("rawURL = %q", rawURL)
	}
	if apiName != "long" {
		t.Fatalf("apiName = %q, want %q", apiName, "long")
	}
	if got := strings.Join(opts.Headers, "\n"); !strings.Contains(got, "X-API: long") {
		t.Fatalf("expected longest-prefix headers, got %q", got)
	}
}

func TestApplyAPIProfileAmbiguousDuplicateBaseURL(t *testing.T) {
	c := New()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"a": {
				BaseURL: "https://api.example.com/v1",
				Profiles: map[string]*config.ProfileConfig{
					"default": {},
				},
			},
			"b": {
				BaseURL: "https://api.example.com/v1",
				Profiles: map[string]*config.ProfileConfig{
					"default": {},
				},
			},
		},
	}

	_, _, _, err := c.applyAPIProfile("https://api.example.com/v1/items", "default", request.Options{}, authHandlerOptions{})
	if err == nil {
		t.Fatal("expected duplicate base URL ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous API match") || !strings.Contains(err.Error(), "a, b") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyAPIProfileMatchesFullBaseURLWithAuthAndNamespace(t *testing.T) {
	c := New()
	c.AddAuthHandler("test", testAuthHandler{})
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"svc": {
				BaseURL: "https://api.example.com/v1",
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Headers: []string{"X-Profile: yes"},
						Auth: &config.AuthConfig{
							Type:   "test",
							Params: map[string]string{"token": "abc"},
						},
					},
				},
			},
		},
	}

	rawURL, apiName, opts, err := c.applyAPIProfile("https://api.example.com/v1/items", "default", request.Options{}, authHandlerOptions{})
	if err != nil {
		t.Fatalf("applyAPIProfile() error = %v", err)
	}
	if rawURL != "https://api.example.com/v1/items" {
		t.Fatalf("rawURL = %q", rawURL)
	}
	if apiName != "svc" {
		t.Fatalf("apiName = %q, want svc", apiName)
	}
	if got := opts.CacheNamespace; got != "svc:default" {
		t.Fatalf("CacheNamespace = %q, want svc:default", got)
	}
	req, _ := http.NewRequest(http.MethodGet, rawURL, nil)
	if err := opts.OnRequest(req); err != nil {
		t.Fatalf("OnRequest() error = %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer abc" {
		t.Fatalf("Authorization = %q, want bearer token", got)
	}
}

func TestApplyAPIProfileMatchesProfileBaseURL(t *testing.T) {
	c := New()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"svc": {
				BaseURL: "https://prod.example.com",
				Profiles: map[string]*config.ProfileConfig{
					"staging": {
						BaseURL: "https://staging.example.com/api",
						Headers: []string{"X-Stage: yes"},
					},
				},
			},
		},
	}

	_, apiName, opts, err := c.applyAPIProfile("https://staging.example.com/api/items", "staging", request.Options{}, authHandlerOptions{})
	if err != nil {
		t.Fatalf("applyAPIProfile() error = %v", err)
	}
	if apiName != "svc" {
		t.Fatalf("apiName = %q, want svc", apiName)
	}
	if got := strings.Join(opts.Headers, "\n"); !strings.Contains(got, "X-Stage: yes") {
		t.Fatalf("expected staging profile headers, got %q", got)
	}
}

func TestApplyAPIProfilePreservesMixedCaseProfileQuery(t *testing.T) {
	c := New()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"svc": {
				BaseURL: "https://api.example.com",
				Profiles: map[string]*config.ProfileConfig{
					"default": {
						Query: []string{"camelCase=value", "X-ID=123"},
					},
				},
			},
		},
	}

	_, _, opts, err := c.applyAPIProfile("svc/items", "default", request.Options{}, authHandlerOptions{})
	if err != nil {
		t.Fatalf("applyAPIProfile() error = %v", err)
	}
	got := strings.Join(opts.Query, ",")
	if !strings.Contains(got, "camelCase=value") || !strings.Contains(got, "X-ID=123") {
		t.Fatalf("profile query = %q, want original case preserved", got)
	}
}

func TestApplyAPIProfileRejectsHostAndPathLookalikes(t *testing.T) {
	c := New()
	c.cfg = &config.Config{
		APIs: map[string]*config.APIConfig{
			"svc": {
				BaseURL:       "https://api.example.com/v1",
				OperationBase: "/api",
				Profiles: map[string]*config.ProfileConfig{
					"default": {Headers: []string{"X-Profile: yes"}},
				},
			},
		},
	}

	for _, rawURL := range []string{
		"https://api.example.com.evil/v1/items",
		"https://api.example.com/v10/items",
		"https://api.example.com/apis/items",
	} {
		_, apiName, opts, err := c.applyAPIProfile(rawURL, "default", request.Options{}, authHandlerOptions{})
		if err != nil {
			t.Fatalf("applyAPIProfile(%q) error = %v", rawURL, err)
		}
		if apiName != "" || len(opts.Headers) != 0 {
			t.Fatalf("applyAPIProfile(%q) matched apiName=%q headers=%v", rawURL, apiName, opts.Headers)
		}
	}
}
