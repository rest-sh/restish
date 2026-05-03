package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
	"github.com/rest-sh/restish/v2/internal/config"
)

// requestRecorder is a small helper that captures the last HTTP request
// a test server received, safe for concurrent access.
type requestRecorder struct {
	mu   sync.Mutex
	last *http.Request
	body []byte
}

func (rr *requestRecorder) capture(r *http.Request) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	// Copy the fields we need; the original request is owned by the server.
	rr.last = &http.Request{
		Method: r.Method,
		URL:    r.URL,
		Host:   r.Host,
		Header: r.Header.Clone(),
	}
	if r.Body != nil {
		rr.body, _ = io.ReadAll(r.Body)
	}
}

func (rr *requestRecorder) Last() *http.Request {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.last
}

func useTransport(c *cli.CLI, fn roundTripperFunc) {
	c.Hooks().HTTPTransport = fn
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}

// TestHTTPVerbs verifies that each lowercase verb sends the correct HTTP method.
func TestHTTPVerbs(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var rr requestRecorder
			c, _, _ := newTestCLI(t)
			useTransport(c, func(r *http.Request) (*http.Response, error) {
				rr.capture(r)
				return jsonResponse(200, `{}`), nil
			})
			if err := c.Run([]string{"restish", strings.ToLower(method), "https://api.example.com/items"}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := rr.Last().Method; got != method {
				t.Errorf("expected method %q, got %q", method, got)
			}
		})
	}
}

// TestHTTPVerbUppercaseAlias verifies that the uppercase alias (e.g. GET) also works.
func TestHTTPVerbUppercaseAlias(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "GET", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Method; got != "GET" {
		t.Errorf("expected GET, got %q", got)
	}
}

// TestBareURL verifies that a URL without an explicit verb is treated as GET.
func TestBareURL(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Method; got != "GET" {
		t.Errorf("expected GET for bare URL, got %q", got)
	}
}

func TestHTTPResponseContentTypeIdentity(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"identity"}},
			Body:       io.NopCloser(strings.NewReader("plain response")),
			Request:    r,
		}, nil
	})
	if err := c.Run([]string{"restish", "-r", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "plain response" {
		t.Fatalf("output = %q, want plain response", got)
	}
}

func TestConfiguredAPIMissingProfileErrors(t *testing.T) {
	cfgData, _ := json.Marshal(&config.Config{
		APIs: map[string]*config.APIConfig{
			"testapi": {
				BaseURL: "https://api.example.com",
				Profiles: map[string]*config.ProfileConfig{
					"default": {},
				},
			},
		},
	})
	cfgFile := t.TempDir() + "/restish.json"
	if err := os.WriteFile(cfgFile, cfgData, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = cfgFile
	err := c.Run([]string{"restish", "get", "--rsh-profile", "missing", "testapi/items"})
	if err == nil {
		t.Fatal("expected missing configured profile to error")
	}
	if !strings.Contains(err.Error(), `profile "missing" not found for API "testapi"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHTTPHeader verifies that -H adds the header to the request.
func TestHTTPHeader(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "get", "-H", "X-Test: hello", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("X-Test"); got != "hello" {
		t.Errorf("expected X-Test=hello, got %q", got)
	}
}

func TestHTTPHostHeader(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "get", "-H", "Host: tenant.example.com", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Host; got != "tenant.example.com" {
		t.Errorf("expected Host tenant.example.com, got %q", got)
	}
}

// TestHTTPHeaderRepeatable verifies that multiple -H flags all take effect.
func TestHTTPHeaderRepeatable(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	err := c.Run([]string{"restish", "get",
		"-H", "X-First: one",
		"-H", "X-Second: two",
		"https://api.example.com/items",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := rr.Last()
	if got := req.Header.Get("X-First"); got != "one" {
		t.Errorf("expected X-First=one, got %q", got)
	}
	if got := req.Header.Get("X-Second"); got != "two" {
		t.Errorf("expected X-Second=two, got %q", got)
	}
}

// TestHTTPQuery verifies that -q appends a query parameter to the request.
func TestHTTPQuery(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "get", "-q", "foo=bar", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.Query().Get("foo"); got != "bar" {
		t.Errorf("expected query foo=bar, got %q", got)
	}
}

func TestHTTPAcceptHeaderOverride(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "get", "-H", "Accept: application/json", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	values := rr.Last().Header.Values("Accept")
	if len(values) != 1 || values[0] != "application/json" {
		t.Fatalf("Accept headers = %#v, want only application/json", values)
	}
}

func TestHTTPHeaderEnvSplitsCommaSeparatedValues(t *testing.T) {
	t.Setenv("RSH_HEADER", "X-One: 1,X-Two: value:with:colons")
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().Header.Get("X-One"); got != "1" {
		t.Fatalf("X-One = %q", got)
	}
	if got := rr.Last().Header.Get("X-Two"); got != "value:with:colons" {
		t.Fatalf("X-Two = %q", got)
	}
}

// TestHTTPServerOverride verifies that -s replaces the scheme and host.
func TestHTTPServerOverride(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	// The URL argument points nowhere meaningful; -s redirects to our test server.
	err := c.Run([]string{"restish", "get", "-s", "https://staging.example.com", "https://api.example.com/items"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.Path; got != "/items" {
		t.Errorf("expected path /items after server override, got %q", got)
	}
}

func TestHTTPServerOverridePrefixesPath(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	err := c.Run([]string{"restish", "get", "-s", "https://staging.example.com/v2", "https://api.example.com/items"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := rr.Last().URL.String(); got != "https://staging.example.com/v2/items" {
		t.Errorf("URL = %q, want override path prefix", got)
	}
}

// TestHTTPResponseBody verifies that the response body is written to stdout.
// Uses a JSON content-type so the body is decoded and re-encoded as an object.
func TestHTTPResponseBody(t *testing.T) {
	c, out, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return jsonResponse(200, `{"hello":"world"}`), nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"hello"`) {
		t.Errorf("expected response body in stdout, got: %q", out.String())
	}
}

// TestHTTPTimeout verifies that --rsh-timeout causes the request to fail
// when the server is too slow.
func TestHTTPTimeout(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	err := c.Run([]string{"restish", "get", "--rsh-timeout", "50ms", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "network") {
		t.Errorf("expected 'network' in error, got: %v", err)
	}
}

func TestHTTPTimeoutShorthand(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	err := c.Run([]string{"restish", "get", "-t", "50ms", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHTTPTimeoutCoversBodyRead(t *testing.T) {
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(readerFunc(func([]byte) (int, error) {
				<-r.Context().Done()
				return 0, r.Context().Err()
			})),
			Request: r,
		}, nil
	})
	err := c.Run([]string{"restish", "get", "--rsh-timeout", "50ms", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected body read timeout")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("expected deadline exceeded error, got %v", err)
	}
}

func TestHTTPDefaultUserAgentAndOverride(t *testing.T) {
	var got []string
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		got = append(got, r.Header.Get("User-Agent"))
		return jsonResponse(200, `{"ok":true}`), nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get default UA: %v", err)
	}

	c2, _, _ := newTestCLI(t)
	useTransport(c2, func(r *http.Request) (*http.Response, error) {
		got = append(got, r.Header.Get("User-Agent"))
		return jsonResponse(200, `{"ok":true}`), nil
	})
	if err := c2.Run([]string{"restish", "get", "-H", "User-Agent: custom", "https://api.example.com/items"}); err != nil {
		t.Fatalf("get custom UA: %v", err)
	}

	if len(got) != 2 || !strings.HasPrefix(got[0], "restish/") || got[1] != "custom" {
		t.Fatalf("User-Agent values = %v", got)
	}
}

// TestHTTPInsecure verifies that --rsh-insecure disables TLS verification.
// The test server uses TLS; without --rsh-insecure the request should fail,
// with it the request should succeed.
func TestHTTPInsecure(t *testing.T) {
	var rr requestRecorder
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rr.capture(r)
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)

	// Without --rsh-insecure: TLS verification fails.
	c, _, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "get", srv.URL}); err == nil {
		t.Error("expected TLS error without --rsh-insecure, got nil")
	}

	// With --rsh-insecure: request succeeds.
	c2, _, _ := newTestCLI(t)
	if err := c2.Run([]string{"restish", "get", "--rsh-insecure", srv.URL}); err != nil {
		t.Errorf("unexpected error with --rsh-insecure: %v", err)
	}
}

// TestShorthandBody verifies that positional args are parsed as shorthand and
// sent as a JSON body with the correct Content-Type.
func TestShorthandBody(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	// Shell would split "name: Alice, age: 30" into tokens; simulate that here.
	if err := c.Run([]string{"restish", "post", "https://api.example.com/items", "name:", "Alice,", "age:", "30"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := rr.Last()
	ct := req.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.body, &body); err != nil {
		t.Fatalf("body is not valid JSON: %v — body: %s", err, rr.body)
	}
	if body["name"] != "Alice" {
		t.Errorf("name: got %v, want Alice", body["name"])
	}
}

// TestShorthandBodyNested verifies deep shorthand paths.
func TestShorthandBodyNested(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "post", "https://api.example.com/items", "user.address.city:", "NYC"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.body, &body); err != nil {
		t.Fatalf("body is not valid JSON: %v — body: %s", err, rr.body)
	}
	user, _ := body["user"].(map[string]any)
	addr, _ := user["address"].(map[string]any)
	if addr["city"] != "NYC" {
		t.Errorf("city: got %v, want NYC", addr["city"])
	}
}

// TestNoBodyWhenNoArgs verifies that GET requests with no positional args
// send no body and no Content-Type.
func TestNoBodyWhenNoArgs(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(rr.body) != 0 {
		t.Errorf("expected no body for GET with no args, got %q", rr.body)
	}
	if ct := rr.Last().Header.Get("Content-Type"); ct != "" {
		t.Errorf("expected no Content-Type for GET with no args, got %q", ct)
	}
}

// TestStdinBody verifies that piped stdin is sent as the request body.
func TestStdinBody(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	c.Stdin = strings.NewReader(`{"from":"stdin"}`)
	if err := c.Run([]string{"restish", "post", "https://api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.body, &body); err != nil {
		t.Fatalf("body not valid JSON: %v — body: %s", err, rr.body)
	}
	if body["from"] != "stdin" {
		t.Errorf("from: got %v, want stdin", body["from"])
	}
}

func TestFormBody(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	err := c.Run([]string{
		"restish", "post", "-c", "form", "https://api.example.com/items",
		"username:", "alice,", "password:", "secret",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := rr.Last()
	if got := req.Header.Get("Content-Type"); !strings.Contains(got, "application/x-www-form-urlencoded") {
		t.Fatalf("expected form content type, got %q", got)
	}
	body := string(rr.body)
	if body != "password=secret&username=alice" && body != "username=alice&password=secret" {
		t.Fatalf("unexpected form body: %q", body)
	}
}

func TestMultipartBody(t *testing.T) {
	var rr requestRecorder
	c, _, _ := newTestCLI(t)
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{}`), nil
	})
	uploadPath := filepath.Join("testdata", "upload.txt")
	err := c.Run([]string{
		"restish", "post", "-c", "multipart", "https://api.example.com/items",
		"name:", "alice,", "file:", "@" + uploadPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	req := rr.Last()
	contentType := req.Header.Get("Content-Type")
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse media type: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}

	reader := multipart.NewReader(bytes.NewReader(rr.body), params["boundary"])
	parts := map[string]string{}
	filenames := map[string]string{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		content, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		parts[part.FormName()] = string(content)
		filenames[part.FormName()] = part.FileName()
	}

	if parts["name"] != "alice" {
		t.Fatalf("name part: got %q", parts["name"])
	}
	if parts["file"] != "hello from upload\n" {
		t.Fatalf("file part: got %q", parts["file"])
	}
	if filenames["file"] != "upload.txt" {
		t.Fatalf("expected upload.txt filename, got %q", filenames["file"])
	}
}
