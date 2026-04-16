package cli_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/restish/v2/internal/cli"
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
	c.RetryBaseDelay = time.Millisecond
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

func TestUnknownCommand(t *testing.T) {
	c, _, _ := newTestCLI()
	if err := c.Run([]string{"restish", "no-such-command"}); err == nil {
		t.Error("expected error for unknown command, got nil")
	}
}

func TestRun_UsesInjectedHTTPTransport(t *testing.T) {
	c, out, _ := newTestCLI()
	c.HTTPTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
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
