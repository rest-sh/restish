package restish_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2"
)

type testAuth struct{}

func TestPublicFetchOptions(test *testing.T) {
	test.Setenv("RSH_CONFIG_DIR", test.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Request") != "public-options" {
			test.Errorf("public fetch did not apply headers: %v", request.Header)
		}
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"public":true}`)
	}))
	defer server.Close()
	client := restish.New()
	response, err := client.FetchResponseWithOptions(context.Background(), http.MethodGet, server.URL, restish.FetchOptions{
		Headers: []string{"X-Request: public-options"}, NoBrowser: true,
	})
	if err != nil || response.Status != http.StatusOK || string(response.Raw) != `{"public":true}` {
		test.Fatalf("public fetch options: response=%v err=%v", response, err)
	}
}

func (testAuth) Parameters() []restish.AuthParam {
	return []restish.AuthParam{{Name: "token", Required: true, Secret: true}}
}

func (testAuth) Authenticate(_ context.Context, req *http.Request, ac restish.AuthContext) error {
	req.Header.Set("Authorization", "Bearer "+ac.Params["token"])
	return nil
}

type testFormatter struct{}

func (testFormatter) Format(w io.Writer, resp *restish.Response, _ bool) error {
	_, err := fmt.Fprint(w, resp.Body)
	return err
}

type testLinkParser struct{}

func (testLinkParser) ParseLinks(baseURL *url.URL, _ http.Header, _ any) []restish.Link {
	return []restish.Link{{Rel: "self", URI: baseURL.String()}}
}

type testLoader struct{}

func (testLoader) Detect(string, []byte) bool { return false }
func (testLoader) LoadWithOptions([]byte, restish.LoadOptions) (*restish.APISpec, error) {
	return nil, nil
}
func (testLoader) Priority() int { return 1000 }

func TestPublicAPIEmbeddableCLI(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RSH_CONFIG_DIR", configDir)
	c := restish.New()
	c.Stdin = bytes.NewReader(nil)
	c.Stdout = &bytes.Buffer{}
	c.Stderr = &bytes.Buffer{}
	c.SetCommandName("acme")
	c.SetCommandDescription("Acme API CLI", "Acme API CLI long help.")
	c.SetVersion("acme-dev")
	c.SetDefaultConfig(&restish.Config{
		APIs: map[string]*restish.APIConfig{
			"acme": {BaseURL: "https://api.example.com"},
		},
	})
	c.AddAuthHandler("test-auth", testAuth{})
	c.AddContentType(&restish.ContentType{Name: "example"})
	c.AddFormatter("test", testFormatter{})
	c.AddLinkParser(testLinkParser{})
	c.AddLoader(testLoader{})
	if err := os.WriteFile(filepath.Join(configDir, "restish.json"), []byte(`{"apis":{"user":{"base_url":"https://user.example.com"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if restish.Version() == "" {
		t.Fatal("expected version")
	}
	if err := c.Run([]string{"acme", "version"}); err != nil {
		t.Fatalf("run version: %v", err)
	}
	if got := c.Stdout.(*bytes.Buffer).String(); !strings.Contains(got, "acme-dev") {
		t.Fatalf("version output = %q, want custom version", got)
	}
	if c.Config().APIs["acme"] == nil || c.Config().APIs["user"] == nil {
		t.Fatalf("default and user APIs should both be present: %#v", c.Config().APIs)
	}
}
