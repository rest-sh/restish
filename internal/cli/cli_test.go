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
func newTestCLI() (*cli.CLI, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
	c.Hooks().RetryBaseDelay = time.Millisecond
	dir, err := os.MkdirTemp("", "restish-cli-test-*")
	if err == nil {
		c.Hooks().ConfigPath = filepath.Join(dir, "restish.json")
	}
	return c, &stdout, &stderr
}

func TestVersion(t *testing.T) {
	c, out, _ := newTestCLI()
	if err := c.Run([]string{"restish", "--version"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "2.0.0") {
		t.Errorf("expected version output to contain '2.0.0', got: %q", out.String())
	}
}

func TestHelp(t *testing.T) {
	c, out, _ := newTestCLI()
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
	c, out, _ := newTestCLI()
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
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "no-such-command"}); err == nil {
		t.Error("expected error for unknown command, got nil")
	}
}

func TestRun_UsesInjectedHTTPTransport(t *testing.T) {
	c, out, _ := newTestCLI()
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
	c, out, _ := newTestCLI()
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "--rsh-retry int   Maximum retry attempts for network errors and 5xx responses (-1 = use default of 2; 0 = disable)") {
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

	c, _, errOut := newTestCLI()
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
