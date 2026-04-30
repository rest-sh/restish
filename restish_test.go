package restish_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/rest-sh/restish/v2"
)

type testAuth struct{}

func (testAuth) Parameters() []restish.AuthParam {
	return []restish.AuthParam{{Name: "token", Required: true, Secret: true}}
}

func (testAuth) Authenticate(_ context.Context, req *http.Request, ac restish.AuthContext) error {
	req.Header.Set("Authorization", "Bearer "+ac.Params["token"])
	return nil
}

func TestPublicAPIEmbeddableCLI(t *testing.T) {
	c := restish.New()
	c.AddAuthHandler("test-auth", testAuth{})
	c.AddContentType(&restish.ContentType{Name: "example"})
	if restish.Version() == "" {
		t.Fatal("expected version")
	}
}
