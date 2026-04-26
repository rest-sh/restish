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

// newTestCLI returns a CLI wired to in-memory buffers for use in tests.
// RetryBaseDelay is set to 1 ms so retry backoffs don't slow down the suite.
func newTestCLI(t *testing.T) (*cli.CLI, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().RetryBaseDelay = time.Millisecond
	c.Hooks().ConfigPath = filepath.Join(t.TempDir(), "restish.json")
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
		"Request Options",
		"Output Options",
		"TLS Options",
		"Pagination and Streaming Options",
		"Cache and Retry Options",
		"General Options",
		"zapi",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected grouped help to contain %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Additional Commands:") {
		t.Fatalf("expected all top-level commands to be grouped:\n%s", got)
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
	if err := c.Run([]string{"restish", "--rsh-config", cfgPath, "api", "add", "myapi", "https://api.example.com"}); err != nil {
		t.Fatalf("api add: %v", err)
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
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "default -1") || strings.Contains(out.String(), "(default -1)") {
		t.Fatalf("help leaked sentinel retry value:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Maximum retry attempts for network errors and 5xx responses (default: 2; 0 = disable)") {
		t.Fatalf("expected user-facing retry default in help, got:\n%s", out.String())
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
