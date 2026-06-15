package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rest-sh/restish/v2/internal/cli"
	"github.com/rest-sh/restish/v2/config"
)

type testApp struct {
	T      *testing.T
	CLI    *cli.CLI
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	c, out, errOut := newTestCLI(t)
	return &testApp{T: t, CLI: c, Stdout: out, Stderr: errOut}
}

func (a *testApp) Run(args ...string) {
	a.T.Helper()
	if err := a.CLI.Run(testArgs(args...)); err != nil {
		a.T.Fatalf("%s: %v", strings.Join(args, " "), err)
	}
}

func (a *testApp) RunErr(args ...string) error {
	a.T.Helper()
	return a.CLI.Run(testArgs(args...))
}

func (a *testApp) UseTransport(fn roundTripperFunc) {
	a.T.Helper()
	useTransport(a.CLI, fn)
}

func (a *testApp) UseResponse(resp *http.Response) {
	a.T.Helper()
	a.UseTransport(func(r *http.Request) (*http.Response, error) {
		if resp.Request == nil {
			resp.Request = r
		}
		return resp, nil
	})
}

func (a *testApp) UseJSONResponse(status int, body string) {
	a.T.Helper()
	a.UseTransport(func(r *http.Request) (*http.Response, error) {
		resp := jsonResponse(status, body)
		resp.Request = r
		return resp, nil
	})
}

func (a *testApp) UseTextResponse(status int, contentType, body string) {
	a.T.Helper()
	a.UseTransport(func(r *http.Request) (*http.Response, error) {
		return textResponse(status, contentType, body, r), nil
	})
}

func (a *testApp) UseOpenAPISpec(specBody string) {
	a.T.Helper()
	useOpenAPISpecTransport(a.CLI, specBody)
}

func (a *testApp) WriteConfig(content string) string {
	a.T.Helper()
	writeTestFile(a.T, a.CLI.Hooks().ConfigPath, content)
	return a.CLI.Hooks().ConfigPath
}

func (a *testApp) WriteConfigObject(cfg *config.Config) string {
	a.T.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		a.T.Fatalf("marshal config: %v", err)
	}
	return a.WriteConfig(string(data))
}

func (a *testApp) ReadConfig() *config.Config {
	a.T.Helper()
	cfg, err := config.Load(a.CLI.Hooks().ConfigPath)
	if err != nil {
		a.T.Fatalf("load config: %v", err)
	}
	return cfg
}

func (a *testApp) SetConfigPath(path string) {
	a.T.Helper()
	a.CLI.Hooks().ConfigPath = path
}

func (a *testApp) FreshConfigPath() string {
	a.T.Helper()
	a.CLI.Hooks().ConfigPath = filepath.Join(a.T.TempDir(), "restish.json")
	return a.CLI.Hooks().ConfigPath
}

func (a *testApp) SetStdoutTTY(v bool) {
	a.T.Helper()
	a.CLI.Hooks().StdoutIsTerminal = func(io.Writer) bool { return v }
}

func testArgs(args ...string) []string {
	out := make([]string, 0, len(args)+1)
	out = append(out, "restish")
	out = append(out, args...)
	return out
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeTestConfig(t *testing.T, cfg *config.Config) string {
	t.Helper()
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "restish.json")
	writeTestFile(t, path, string(data))
	return path
}

func writeAPIConfigObject(t *testing.T, name string, api *config.APIConfig) string {
	t.Helper()
	return writeTestConfig(t, &config.Config{APIs: map[string]*config.APIConfig{name: api}})
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func useOpenAPISpecTransport(c *cli.CLI, specBody string) {
	useOpenAPISpecTransportFunc(c, func() string { return specBody })
}

func useJSONResponse(c *cli.CLI, status int, body string) {
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		resp := jsonResponse(status, body)
		resp.Request = r
		return resp, nil
	})
}

func useTextResponse(c *cli.CLI, status int, contentType, body string) {
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		return textResponse(status, contentType, body, r), nil
	})
}

func useOpenAPISpecTransportFunc(c *cli.CLI, specBody func() string) {
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case "https://api.example.com":
			return &http.Response{
				StatusCode: 200,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    r,
			}, nil
		case "https://api.example.com/openapi.json":
			return textResponse(200, "application/json", specBody(), r), nil
		default:
			return &http.Response{
				StatusCode: 404,
				Proto:      "HTTP/1.1",
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader("not found")),
				Request:    r,
			}, nil
		}
	})
}

func requireContains(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func requireNotContains(t *testing.T, got string, rejects ...string) {
	t.Helper()
	for _, reject := range rejects {
		if strings.Contains(got, reject) {
			t.Fatalf("unexpected %q:\n%s", reject, got)
		}
	}
}

func testAPIConfig(baseURL string, profile *config.ProfileConfig) *config.APIConfig {
	api := &config.APIConfig{BaseURL: baseURL}
	if profile != nil {
		api.Profiles = map[string]*config.ProfileConfig{"default": profile}
	}
	return api
}

func profileAuth(auth *config.AuthConfig) *config.ProfileConfig {
	return &config.ProfileConfig{Auth: auth}
}

func profileCredentials(credentials map[string]*config.CredentialConfig) *config.ProfileConfig {
	return &config.ProfileConfig{Credentials: credentials}
}

func apiKeyAuth(in, name, value string) *config.AuthConfig {
	return &config.AuthConfig{Type: "api-key", Params: map[string]string{"in": in, "name": name, "value": value}}
}

func basicAuth(username, password string) *config.AuthConfig {
	return &config.AuthConfig{Type: "http-basic", Params: map[string]string{"username": username, "password": password}}
}

func bearerAuth(token string) *config.AuthConfig {
	return &config.AuthConfig{Type: "bearer", Params: map[string]string{"token": token}}
}

func testCredential(auth *config.AuthConfig, satisfies ...string) *config.CredentialConfig {
	return &config.CredentialConfig{Auth: auth, Satisfies: satisfies}
}

func openAPISpec(baseURL, title string, sections ...string) string {
	fields := []string{
		`"openapi":"3.1.0"`,
		fmt.Sprintf(`"info":{"title":%q,"version":"1.0"}`, title),
		fmt.Sprintf(`"servers":[{"url":%q}]`, baseURL),
	}
	fields = append(fields, sections...)
	return "{" + strings.Join(fields, ",") + "}"
}

func openAPIGetSpec(baseURL, title, path, operationID string, sections ...string) string {
	return openAPISpec(baseURL, title, append(sections, openAPIPaths(openAPIGet(path, operationID)))...)
}

func openAPIGetOperationSpec(baseURL, title, path, operationID string, fields ...string) string {
	return openAPISpec(baseURL, title, openAPIPaths(openAPIGet(path, operationID, fields...)))
}

func openAPIPaths(paths ...string) string {
	return `"paths":{` + strings.Join(paths, ",") + `}`
}

func openAPISecuritySchemes(schemes ...string) string {
	return `"components":{"securitySchemes":{` + strings.Join(schemes, ",") + `}}`
}

func openAPISecurity(requirements ...string) string {
	return `"security":[` + strings.Join(requirements, ",") + `]`
}

func openAPIGet(path, operationID string, fields ...string) string {
	return openAPIOperation(path, "get", operationID, fields...)
}

func openAPIParams(params ...string) string {
	return `"parameters":[` + strings.Join(params, ",") + `]`
}

func openAPIParam(name, in string, required bool, schema string, fields ...string) string {
	paramFields := []string{fmt.Sprintf(`"name":%q`, name), fmt.Sprintf(`"in":%q`, in)}
	if required {
		paramFields = append(paramFields, `"required":true`)
	}
	if schema != "" {
		paramFields = append(paramFields, `"schema":`+schema)
	}
	paramFields = append(paramFields, fields...)
	return "{" + strings.Join(paramFields, ",") + "}"
}

func openAPIOperation(path, method, operationID string, fields ...string) string {
	var opFields []string
	if operationID != "" {
		opFields = append(opFields, fmt.Sprintf(`"operationId":%q`, operationID))
	}
	opFields = append(opFields, fields...)
	opFields = append(opFields, `"responses":{"200":{"description":"OK"}}`)
	return fmt.Sprintf(`%q:{%q:{%s}}`, path, method, strings.Join(opFields, ","))
}

func textResponse(status int, contentType, body string, req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: status,
		Proto:      "HTTP/1.1",
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}
