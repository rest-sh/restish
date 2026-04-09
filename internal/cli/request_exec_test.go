package cli

import (
	"io"
	"net/http"
	"strings"
	"testing"

	authpkg "github.com/danielgtaylor/restish/v2/auth"
	"github.com/danielgtaylor/restish/v2/internal/config"
	"github.com/danielgtaylor/restish/v2/internal/request"
)

type testAuthHandler struct{}

func (testAuthHandler) Parameters() []authpkg.Param { return nil }

func (testAuthHandler) OnRequest(req *http.Request, params map[string]string) error {
	req.Header.Set("Authorization", "Bearer "+params["token"])
	return nil
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

	prepared, err := c.prepareRequest("svc/items", "default", request.Options{}, nil, nil, true)
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
