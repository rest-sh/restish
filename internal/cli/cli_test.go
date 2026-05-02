package cli_test

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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

func TestDoctorReportsInvalidConfigWithoutFailing(t *testing.T) {
	c, _, errOut := newTestCLI(t)
	if err := os.WriteFile(c.Hooks().ConfigPath, []byte("{\n  \"apiss\": {}\n}"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := c.Run([]string{"restish", "doctor"}); err != nil {
		t.Fatalf("doctor returned error with invalid config: %v", err)
	}
	got := errOut.String()
	if !strings.Contains(got, "Config parse: invalid") ||
		!strings.Contains(got, c.Hooks().ConfigPath) ||
		!strings.Contains(got, "did you mean \"apis\"") {
		t.Fatalf("unexpected doctor output:\n%s", got)
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
}

func TestUnknownCommand(t *testing.T) {
	c, _, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "no-such-command"}); err == nil {
		t.Error("expected error for unknown command, got nil")
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
	if !strings.Contains(err.Error(), "explicit config file") {
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
	if !strings.Contains(out.String(), "Maximum retry attempts for network errors and transient HTTP responses (default: 2; 0 = disable)") {
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
