package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/rest-sh/restish/v2/internal/filter"
	"github.com/spf13/cobra"
)

func newFilterTestCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("rsh-raw", false, "")
	// Simulate PersistentPreRunE: parse flags and store GlobalFlags on context.
	gf, err := parseGlobalFlags(cmd)
	if err != nil {
		t.Fatalf("parseGlobalFlags: %v", err)
	}
	cmd.SetContext(withGlobalFlags(context.Background(), gf))
	return cmd
}

func TestFilterOutputReturnsFilteredValue(t *testing.T) {
	var stdout bytes.Buffer
	c := &CLI{Stdout: &stdout}
	cmd := newFilterTestCommand(t)

	filtered, handled, err := c.filterOutput(cmd, "body.answer", map[string]any{
		"body": map[string]any{"answer": 42},
	}, filter.LangAuto)
	if err != nil {
		t.Fatalf("filterOutput: %v", err)
	}
	if handled {
		t.Fatal("expected non-raw mode to leave output handling to caller")
	}
	if filtered != 42 {
		t.Fatalf("filtered = %#v, want 42", filtered)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout in non-raw mode: %q", stdout.String())
	}
}
