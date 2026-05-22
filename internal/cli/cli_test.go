package cli_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rest-sh/restish/v2/internal/cli"
	"github.com/rest-sh/restish/v2/internal/config"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

var testPluginManifestCachePath string

// newTestCLI returns a CLI wired to in-memory buffers for use in tests.
// RetryBaseDelay is set to 1 ms so retry backoffs don't slow down the suite.
func newTestCLI(t *testing.T) (*cli.CLI, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().PassReader = strings.NewReader("")
	c.Hooks().RetryBaseDelay = time.Millisecond
	stateDir := t.TempDir()
	if configDir := os.Getenv("RSH_CONFIG_DIR"); configDir != "" {
		stateDir = configDir
	}
	c.Hooks().ConfigPath = filepath.Join(stateDir, "restish.json")
	c.Hooks().TokenCachePath = filepath.Join(stateDir, "tokens.cbor")
	c.Hooks().CachePath = filepath.Join(stateDir, "http-cache")
	c.Hooks().SpecCachePath = filepath.Join(stateDir, "spec-cache")
	if testPluginManifestCachePath != "" {
		c.Hooks().PluginManifestCachePath = testPluginManifestCachePath
	} else {
		c.Hooks().PluginManifestCachePath = filepath.Join(stateDir, "plugin-manifest.cbor")
	}
	return c, &stdout, &stderr
}

func TestVersion(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "--version"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "2.0.0") {
		t.Errorf("expected version output to contain '2.0.0', got: %q", out.String())
	}
}

func TestVersionCommand(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "version"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "2.0.0") {
		t.Errorf("expected version output to contain '2.0.0', got: %q", out.String())
	}
}

func TestSpecBackedShortNameSyncsGeneratedCommandBeforeGenericFallback(t *testing.T) {
	c, _, _ := newTestCLI(t)
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	specBody := `openapi: "3.1.0"
info:
  title: Test
  version: "1.0.0"
paths:
  /ping:
    get:
      operationId: getPing
      responses:
        "200":
          description: OK
`
	if err := os.WriteFile(specPath, []byte(specBody), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	configBody := `{"apis":{"svc":{"base_url":"https://api.example.com","spec_files":[` + strconv.Quote(specPath) + `]}}}`
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	var rr requestRecorder
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		rr.capture(r)
		return jsonResponse(200, `{"ok":true}`), nil
	})
	if err := c.Run([]string{"restish", "svc", "get-ping"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := rr.Last()
	if req == nil {
		t.Fatal("expected request")
	}
	if got := req.Method; got != "GET" {
		t.Fatalf("method = %q, want GET", got)
	}
	if got := req.URL.Path; got != "/ping" {
		t.Fatalf("path = %q, want /ping", got)
	}
}

func TestHelp(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	for _, want := range []string{"restish", "HTTP"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected help output to contain %q:\n%s", want, got)
		}
	}
}

func TestHelpColorizesWhenColorEnabled(t *testing.T) {
	t.Setenv("COLOR", "1")
	t.Setenv("NO_COLOR", "")
	t.Setenv("NOCOLOR", "")

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected colored help output, got:\n%s", got)
	}
	plain := stripANSI(got)
	for _, want := range []string{"Usage:", "Generic HTTP Commands", "get"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("colored help should preserve %q in plain text:\n%s", want, plain)
		}
	}
}

func TestHelpHidesRequestFlagsForNonRequestCommands(t *testing.T) {
	for _, args := range [][]string{
		{"restish", "shell", "setup", "--help"},
		{"restish", "plugin", "--help"},
		{"restish", "cache", "--help"},
		{"restish", "config", "--help"},
		{"restish", "api", "list", "--help"},
		{"restish", "api", "remove", "--help"},
		{"restish", "config", "theme", "--help"},
	} {
		c, out, _ := newTestCLI(t)
		if err := c.Run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		got := out.String()
		for _, hidden := range []string{"--rsh-header", "--rsh-output-format", "--rsh-no-paginate", "--rsh-insecure"} {
			if strings.Contains(got, hidden) {
				t.Fatalf("%v should omit request global %s by default:\n%s", args, hidden, got)
			}
		}
		for _, visible := range []string{"--help-all", "--rsh-config", "--rsh-verbose"} {
			if !strings.Contains(got, visible) {
				t.Fatalf("%v should keep core global %s visible:\n%s", args, visible, got)
			}
		}
	}
}

func TestRequestHelpShowsRequestFlagsAndHelpAllExpandsNonRequestHelp(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "get", "--help"}); err != nil {
		t.Fatalf("get --help: %v", err)
	}
	got := out.String()
	for _, want := range []string{"--rsh-header", "--rsh-output-format", "--rsh-no-paginate"} {
		if !strings.Contains(got, want) {
			t.Fatalf("request help should show %s:\n%s", want, got)
		}
	}

	c, out, _ = newTestCLI(t)
	if err := c.Run([]string{"restish", "shell", "setup", "--help-all", "--help"}); err != nil {
		t.Fatalf("setup --help-all --help: %v", err)
	}
	got = out.String()
	for _, want := range []string{"--rsh-header", "--rsh-output-format", "--rsh-no-paginate"} {
		if !strings.Contains(got, want) {
			t.Fatalf("help-all should show %s:\n%s", want, got)
		}
	}
}

func TestHelpAllBypassesValidationAndExecution(t *testing.T) {
	for _, args := range [][]string{
		{"restish", "get", "--help-all"},
		{"restish", "api", "connect", "--help-all"},
		{"restish", "links", "--help-all"},
		{"restish", "cert", "--help-all"},
		{"restish", "shell", "setup", "--help-all"},
		{"restish", "config", "theme", "reset", "--help-all"},
		{"restish", "config", "path", "--help-all"},
		{"restish", "config", "show", "--help-all"},
	} {
		c, out, _ := newTestCLI(t)
		if err := c.Run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		got := out.String()
		if !strings.Contains(got, "Usage:") || !strings.Contains(got, "--rsh-header") {
			t.Fatalf("%v should show expanded help, got:\n%s", args, got)
		}
	}

	c, out, _ := newTestCLI(t)
	called := false
	useTransport(c, func(r *http.Request) (*http.Response, error) {
		called = true
		return jsonResponse(200, `{}`), nil
	})
	if err := c.Run([]string{"restish", "get", "https://api.example.com/items", "--help-all"}); err != nil {
		t.Fatalf("get URL --help-all: %v", err)
	}
	if called {
		t.Fatal("help-all after URL should not execute the HTTP request")
	}
	if got := out.String(); !strings.Contains(got, "Perform an HTTP `GET` request") || !strings.Contains(got, "--rsh-header") {
		t.Fatalf("expected GET help-all output, got:\n%s", got)
	}
}

func TestBootstrapCommandsIgnoreInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "help flag", args: []string{"restish", "--help"}},
		{name: "help command", args: []string{"restish", "help"}},
		{name: "version flag", args: []string{"restish", "--version"}},
		{name: "version command", args: []string{"restish", "version"}},
		{name: "completion", args: []string{"restish", "completion", "bash"}},
		{name: "setup help", args: []string{"restish", "shell", "setup", "--help"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _ := newTestCLI(t)
			if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":`), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if err := c.Run(tc.args); err != nil {
				t.Fatalf("%v returned error with invalid config: %v", tc.args, err)
			}
		})
	}
}

func TestRunRejectsInsecureConfigPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits not authoritative on Windows")
	}
	c, _, _ := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{}}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	err := c.Run([]string{"restish", "help"})
	if err == nil {
		t.Fatal("expected insecure config permissions error")
	}
	if !strings.Contains(err.Error(), "chmod 600") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDoctorReportsInvalidConfigWithoutFailing(t *testing.T) {
	c, out, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte("{\n  \"apiss\": {}\n}"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor"}); err != nil {
		t.Fatalf("doctor returned error with invalid config: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Config parse: invalid") ||
		!strings.Contains(got, c.Hooks().ConfigPath) ||
		!strings.Contains(got, "did you mean \"apis\"") {
		t.Fatalf("unexpected doctor output:\n%s", got)
	}
	if !strings.Contains(errOut.String(), "Tip: use -o json for machine-readable output.") {
		t.Fatalf("expected redirected-output JSON hint on stderr, got:\n%s", errOut.String())
	}
}

func TestBuiltInAPINameFailsAtConfigLoad(t *testing.T) {
	c, _, _ := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{"apis":{"get":{"base_url":"https://api.example.com"}}}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	err := c.Run([]string{"restish", "get", "https://api.example.com"})
	if err == nil {
		t.Fatal("expected built-in API name to fail config load")
	}
	if !strings.Contains(err.Error(), `API name "get" conflicts`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHelpGroupsTopLevelCommands(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte(`{
  "apis": {
    "zapi": {
      "base_url": "https://api.example.com"
    }
  }
}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"Generic HTTP Commands",
		"Configuration and Setup",
		"Plugin Commands",
		"Registered APIs",
		"Utilities",
		"Help",
		"General Options",
		"--help-all",
		"--rsh-config",
		"--rsh-verbose",
		"zapi",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected grouped help to contain %q:\n%s", want, got)
		}
	}
	for _, hidden := range []string{
		"Request Options",
		"Output Options",
		"TLS Options",
		"Pagination and Streaming Options",
		"Cache and Retry Options",
		"--rsh-header",
		"--rsh-output-format",
		"--rsh-insecure",
		"--rsh-no-paginate",
	} {
		if strings.Contains(got, hidden) {
			t.Errorf("expected default root help to omit %q:\n%s", hidden, got)
		}
	}
	if strings.Contains(got, "Additional Commands:") {
		t.Fatalf("expected all top-level commands to be grouped:\n%s", got)
	}

	c, out, _ = newTestCLI(t)
	if err := c.Run([]string{"restish", "--help-all", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got = out.String()
	for _, want := range []string{
		"Request Options",
		"Output Options",
		"Auth and Profile Options",
		"TLS Options",
		"Pagination and Streaming Options",
		"Cache and Retry Options",
		"General Options",
		"--rsh-auth",
		"--rsh-config",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected root --help-all to contain %q:\n%s", want, got)
		}
	}
	outputGroupIdx := strings.Index(got, "Output Options")
	printFlagIdx := strings.Index(got, "--rsh-print")
	authGroupIdx := strings.Index(got, "Auth and Profile Options")
	if outputGroupIdx < 0 || printFlagIdx < outputGroupIdx || authGroupIdx < 0 || printFlagIdx > authGroupIdx {
		t.Fatalf("--rsh-print should be grouped under Output Options in --help-all:\n%s", got)
	}
}

func TestUnknownCommand(t *testing.T) {
	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "apis"})
	if err == nil {
		t.Error("expected error for unknown command, got nil")
	} else if !strings.Contains(err.Error(), `did you mean "api"?`) {
		t.Fatalf("expected suggestion for api command, got %v", err)
	}
}

func TestExplicitConfigFlagWritesSelectedFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "project-restish.json")
	if err := os.WriteFile(cfgPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run([]string{"restish", "--rsh-config", cfgPath, "api", "connect", "myapi", "https://api.example.com", "--no-discover"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load explicit config: %v", err)
	}
	if got := cfg.APIs["myapi"].BaseURL; got != "https://api.example.com" {
		t.Fatalf("base_url = %q", got)
	}
}

func TestExplicitConfigFlagConnectCreatesSelectedFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nested", "project-restish.json")

	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run([]string{"restish", "--rsh-config", cfgPath, "api", "connect", "myapi", "https://api.example.com", "--no-discover"}); err != nil {
		t.Fatalf("api connect: %v", err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat explicit config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load explicit config: %v", err)
	}
	if got := cfg.APIs["myapi"].BaseURL; got != "https://api.example.com" {
		t.Fatalf("base_url = %q, want https://api.example.com", got)
	}
}

func TestRSHConfigReadsSelectedFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "project-restish.json")
	if err := os.WriteFile(cfgPath, []byte(`{
  "apis": {
    "project": {"base_url": "https://project.example.com"}
  }
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RSH_CONFIG", cfgPath)

	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run([]string{"restish", "api", "list"}); err != nil {
		t.Fatalf("api list: %v", err)
	}
	if !strings.Contains(stdout.String(), "project.example.com") {
		t.Fatalf("expected RSH_CONFIG API in output, got %q", stdout.String())
	}
}

func TestExplicitConfigMissingErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	missing := filepath.Join(t.TempDir(), "missing.json")
	err := c.Run([]string{"restish", "--rsh-config", missing, "api", "list"})
	if err == nil {
		t.Fatal("expected missing explicit config to error")
	}
	if !strings.Contains(err.Error(), "--rsh-config") ||
		!strings.Contains(err.Error(), "v2 does not fall back to the default config") ||
		!strings.Contains(err.Error(), "create the file or remove the flag") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_UsesInjectedHTTPTransport(t *testing.T) {
	c, out, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.String(), "https://api.example.com/items"; got != want {
			t.Fatalf("URL = %q, want %q", got, want)
		}
		return &http.Response{
			StatusCode: 200,
			Proto:      "HTTP/1.1",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    r,
		}, nil
	})

	if err := c.Run([]string{"restish", "get", "api.example.com/items"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `"ok"`) {
		t.Fatalf("expected response body in stdout, got %q", out.String())
	}
}

func TestHelpDoesNotExposeRetrySentinelValue(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "get", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "default -1") || strings.Contains(out.String(), "(default -1)") {
		t.Fatalf("help leaked sentinel retry value:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Maximum retry attempts for network errors and transient HTTP responses (0 = disable) (default 2)") {
		t.Fatalf("expected user-facing retry default in help, got:\n%s", out.String())
	}
}

func TestInvalidRSHRetryFailsFast(t *testing.T) {
	t.Setenv("RSH_RETRY", "abc")
	c, _, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent with invalid RSH_RETRY")
		return nil, nil
	})
	err := c.Run([]string{"restish", "get", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected invalid RSH_RETRY error")
	}
	if !strings.Contains(err.Error(), "invalid RSH_RETRY") {
		t.Fatalf("expected invalid RSH_RETRY error, got %v", err)
	}
}

func TestNegativeNumericGlobalFlagsFailFast(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "retry",
			args: []string{"restish", "get", "--rsh-retry", "-1", "https://api.example.com/items"},
			want: "invalid --rsh-retry -1",
		},
		{
			name: "max pages",
			args: []string{"restish", "get", "--rsh-max-pages", "-1", "https://api.example.com/items"},
			want: "invalid --rsh-max-pages -1",
		},
		{
			name: "max items",
			args: []string{"restish", "get", "--rsh-max-items", "-1", "https://api.example.com/items"},
			want: "invalid --rsh-max-items -1",
		},
		{
			name: "max body size",
			args: []string{"restish", "get", "--rsh-max-body-size", "-1", "https://api.example.com/items"},
			want: "invalid --rsh-max-body-size -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _, _ := newTestCLI(t)
			c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatal("request should not be sent with invalid numeric global flag")
				return nil, nil
			})
			err := c.Run(tt.args)
			if err == nil {
				t.Fatalf("%v: expected invalid numeric flag error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("%v: expected error containing %q, got %v", tt.args, tt.want, err)
			}
		})
	}
}

func TestNegativeTimeoutGlobalFlagsFailFast(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{
			name: "flag",
			args: []string{"restish", "get", "--rsh-timeout", "-1s", "https://api.example.com/items"},
			want: `invalid --rsh-timeout "-1s"`,
		},
		{
			name: "env",
			args: []string{"restish", "get", "https://api.example.com/items"},
			env:  "-1s",
			want: `invalid RSH_TIMEOUT "-1s"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("RSH_TIMEOUT", tt.env)
			}
			c, _, _ := newTestCLI(t)
			c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatal("request should not be sent with invalid timeout")
				return nil, nil
			})
			err := c.Run(tt.args)
			if err == nil {
				t.Fatalf("%v: expected invalid timeout error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("%v: expected error containing %q, got %v", tt.args, tt.want, err)
			}
		})
	}
}

func TestInvalidFilterLangFailsFast(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent with invalid filter language")
		return nil, nil
	})
	err := c.Run([]string{"restish", "get", "--rsh-filter-lang", "nope", "-f", "body.url", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected invalid filter language error")
	}
	if !strings.Contains(err.Error(), `invalid --rsh-filter-lang "nope"`) {
		t.Fatalf("expected invalid --rsh-filter-lang error, got %v", err)
	}
}

func TestInvalidTLSMinVersionFailsFast(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent with invalid TLS minimum version")
		return nil, nil
	})
	err := c.Run([]string{"restish", "get", "--rsh-tls-min-version", "TLS1.1", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected invalid TLS minimum version error")
	}
	if !strings.Contains(err.Error(), `invalid --rsh-tls-min-version "TLS1.1"`) {
		t.Fatalf("expected invalid --rsh-tls-min-version error, got %v", err)
	}
	if !strings.Contains(err.Error(), "TLS1.2") || !strings.Contains(err.Error(), "TLS1.3") {
		t.Fatalf("expected error to list supported TLS versions, got %v", err)
	}
}

func TestNegativeRSHRetryFailsFast(t *testing.T) {
	t.Setenv("RSH_RETRY", "-1")
	c, _, _ := newTestCLI(t)
	c.Hooks().HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		t.Fatal("request should not be sent with invalid RSH_RETRY")
		return nil, nil
	})
	err := c.Run([]string{"restish", "get", "https://api.example.com/items"})
	if err == nil {
		t.Fatal("expected invalid RSH_RETRY error")
	}
	if !strings.Contains(err.Error(), `invalid RSH_RETRY "-1"`) {
		t.Fatalf("expected invalid RSH_RETRY error, got %v", err)
	}
}

func TestRun_PrintsLegacyMigrationNotice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", home)
		t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	}

	legacyConfigPath := config.DefaultPath()
	legacyDir := filepath.Dir(legacyConfigPath)
	if err := os.MkdirAll(legacyDir, 0o700); err != nil {
		t.Fatalf("mkdir legacy dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "apis.json"), []byte(`{
  "example": {
    "base": "https://api.example.com"
  }
}`), 0o600); err != nil {
		t.Fatalf("write apis.json: %v", err)
	}

	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = legacyConfigPath
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := errOut.String()
	want := "Migrated config from v1 at " + legacyDir + "; kept backup at " + legacyDir + ".bak.v1"
	if !strings.Contains(got, want) {
		t.Fatalf("expected migration notice %q, got:\n%s", want, got)
	}
}
