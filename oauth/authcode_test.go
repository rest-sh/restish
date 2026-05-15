package oauth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeUrlWindowsSuccess(t *testing.T) {
	u := "https://mydomain.auth.us-east-1.amazoncognito.com/oauth2/authorize?response_type=code&client_id=1example23456789&redirect_uri=https://www.example.com&state=abcdefg&scope=openid+profile"

	r := encodeUrlWindows(u)
	//t.Log(r)

	assert.NotEqual(t, u, r)
	assert.Contains(t, r, "^&")
	assert.False(t, strings.HasPrefix(r, "^&"))
	assert.False(t, strings.HasSuffix(r, "^&"))
}

// runAuthHandler drives the authHandler with the given query string and
// returns the response body, allowing assertions on what HTML was served.
func runAuthHandler(t *testing.T, h authHandler, query string) string {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://localhost/callback?"+query, nil)

	// authHandler sends on h.c synchronously; drain in a goroutine so the
	// handler can complete without blocking on an unbuffered channel.
	done := make(chan struct{})
	go func() {
		<-h.c
		close(done)
	}()

	h.ServeHTTP(rec, req)
	<-done

	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	return string(body)
}

func TestAuthHandlerServesDefaultSuccessHTML(t *testing.T) {
	h := authHandler{c: make(chan string)}

	body := runAuthHandler(t, h, "code=abc123")

	assert.Contains(t, body, "Login Successful!", "default success HTML should be served when successHTML is unset")
}

func TestAuthHandlerServesCustomSuccessHTML(t *testing.T) {
	const custom = `<html><body><h1>Welcome to my-tool</h1></body></html>`
	h := authHandler{c: make(chan string), successHTML: custom}

	body := runAuthHandler(t, h, "code=abc123")

	assert.Equal(t, custom, body, "custom successHTML should be served verbatim")
	assert.NotContains(t, body, "Login Successful!", "default content should not leak through")
}

func TestAuthHandlerServesDefaultErrorHTML(t *testing.T) {
	h := authHandler{c: make(chan string)}

	body := runAuthHandler(t, h, "error=access_denied&error_description=user+denied")

	assert.Contains(t, body, "access_denied", "$ERROR should be substituted in the default error HTML")
	assert.Contains(t, body, "user denied", "$DETAILS should be substituted in the default error HTML")
}

func TestAuthHandlerServesCustomErrorHTML(t *testing.T) {
	const custom = `<html><body><h1>Login failed: $ERROR</h1><p>$DETAILS</p></body></html>`
	h := authHandler{c: make(chan string), errorHTML: custom}

	body := runAuthHandler(t, h, "error=invalid_grant&error_description=code+expired")

	assert.Equal(t, `<html><body><h1>Login failed: invalid_grant</h1><p>code expired</p></body></html>`, body,
		"custom errorHTML should be served with $ERROR and $DETAILS substituted")
}

func TestAuthHandlerCustomSuccessDoesNotAffectErrorPath(t *testing.T) {
	// A caller that only overrides successHTML should still see the default
	// error HTML when an error comes through. Each field is independently
	// optional.
	h := authHandler{c: make(chan string), successHTML: "<html>custom success</html>"}

	body := runAuthHandler(t, h, "error=server_error")

	assert.Contains(t, body, "server_error", "default error HTML should still substitute $ERROR")
	assert.NotContains(t, body, "custom success")
}
