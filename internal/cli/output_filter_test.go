package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/danielgtaylor/restish/v2/internal/filter"
	"github.com/spf13/cobra"
)

func newFilterTestCommand(t *testing.T, raw bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("rsh-raw", false, "")
	if err := cmd.Flags().Set("rsh-raw", map[bool]string{true: "true", false: "false"}[raw]); err != nil {
		t.Fatalf("set rsh-raw: %v", err)
	}
	// Simulate PersistentPreRunE: parse flags and store GlobalFlags on context.
	gf := parseGlobalFlags(cmd)
	cmd.SetContext(withGlobalFlags(context.Background(), gf))
	return cmd
}

func TestFilterOutputReturnsFilteredValue(t *testing.T) {
	var stdout bytes.Buffer
	c := &CLI{Stdout: &stdout}
	cmd := newFilterTestCommand(t, false)

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

func TestFilterOutputWritesRawValue(t *testing.T) {
	var stdout bytes.Buffer
	c := &CLI{Stdout: &stdout}
	cmd := newFilterTestCommand(t, true)

	filtered, handled, err := c.filterOutput(cmd, "body.answer", map[string]any{
		"body": map[string]any{"answer": "hello"},
	}, filter.LangAuto)
	if err != nil {
		t.Fatalf("filterOutput: %v", err)
	}
	if !handled {
		t.Fatal("expected raw mode to handle output directly")
	}
	if filtered != nil {
		t.Fatalf("filtered = %#v, want nil after raw output", filtered)
	}
	if got := stdout.String(); got != "hello\n" {
		t.Fatalf("stdout = %q, want %q", got, "hello\n")
	}
}
