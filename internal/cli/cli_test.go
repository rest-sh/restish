package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/cli"
)

// newTestCLI returns a CLI wired to in-memory buffers for use in tests.
func newTestCLI() (*cli.CLI, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	c := cli.New()
	c.Stdin = strings.NewReader("")
	c.Stdout = &stdout
	c.Stderr = &stderr
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
