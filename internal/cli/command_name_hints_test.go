package cli_test

import (
	"strings"
	"testing"
)

// TestCommandNameInHintsAndHelp verifies that user-facing help and hint text
// uses the configured command name (via SetCommandName) rather than the
// hard-coded "restish" binary name. Regression guard for embedders whose
// binary is not named "restish": a hint like `run "restish api sync"` points
// at a binary the user does not have installed.
func TestCommandNameInHintsAndHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		// config show with no configured APIs emits the "api connect" hint.
		{"config-show", []string{"config", "show"}},
		// root help renders rootLongDefault, which references api connect / get / post.
		{"root-help", []string{"--help"}},
		// api set help renders apiSetLong, which references api inspect.
		{"api-set-help", []string{"api", "set", "--help"}},
		// doctor api help renders doctorAPILong, which references api auth inspect.
		{"doctor-api-help", []string{"doctor", "api", "--help"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, out, errOut := newTestCLI(t)
			c.SetCommandName("myapp")
			if err := c.Run(append([]string{"myapp"}, tc.args...)); err != nil {
				t.Fatalf("run %v: %v", tc.args, err)
			}
			combined := out.String() + errOut.String()
			// No hint should tell the user to run the "restish" binary.
			for _, bad := range []string{"restish api ", "restish get", "restish post", "`restish "} {
				if strings.Contains(combined, bad) {
					t.Errorf("output references hard-coded binary %q, got:\n%s", bad, combined)
				}
			}
		})
	}
}

// TestCommandNameDefaultsToRestish confirms the default (no SetCommandName)
// still produces "restish"-branded help, so the stock binary is unchanged.
func TestCommandNameDefaultsToRestish(t *testing.T) {
	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "--help"}); err != nil {
		t.Fatalf("run --help: %v", err)
	}
	if !strings.Contains(out.String(), "restish api connect") {
		t.Errorf("expected default help to reference \"restish api connect\", got:\n%s", out.String())
	}
}
