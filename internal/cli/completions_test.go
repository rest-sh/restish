package cli_test

import (
	"strings"
	"testing"
)

// runCompletion invokes cobra's __complete subcommand and returns stdout.
func runCompletion(t *testing.T, args ...string) string {
	t.Helper()
	c, out, _ := newTestCLI(t)
	full := append([]string{"restish", "__complete"}, args...)
	// Errors from __complete are expected when no match; ignore them.
	_ = c.Run(full)
	return out.String()
}

// TestOutputFormatCompletions verifies that -o / --rsh-output-format lists
// the registered formatter names.
func TestOutputFormatCompletions(t *testing.T) {
	got := runCompletion(t, "get", "--rsh-output-format", "")
	for _, want := range []string{"json", "readable", "raw", "yaml"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in -o completions, got:\n%s", want, got)
		}
	}
}

// TestProfileCompletions verifies that -p / --rsh-profile always returns at
// least "default".
func TestProfileCompletions(t *testing.T) {
	got := runCompletion(t, "get", "--rsh-profile", "")
	if !strings.Contains(got, "default") {
		t.Errorf("expected 'default' in -p completions, got:\n%s", got)
	}
}

// TestContentTypeCompletions verifies that -c / --rsh-content-type returns
// names of the registered content types.
func TestContentTypeCompletions(t *testing.T) {
	got := runCompletion(t, "get", "--rsh-content-type", "")
	for _, want := range []string{"json", "yaml"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in -c completions, got:\n%s", want, got)
		}
	}
}

// TestFilterLangCompletions verifies that --rsh-filter-lang returns exactly
// "shorthand" and "jq".
func TestFilterLangCompletions(t *testing.T) {
	got := runCompletion(t, "get", "--rsh-filter-lang", "")
	for _, want := range []string{"shorthand", "jq"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in --rsh-filter-lang completions, got:\n%s", want, got)
		}
	}
}
