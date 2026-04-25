package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCompletionScripts verifies that Cobra generates non-empty completion
// scripts for each supported shell.
func TestCompletionScripts(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			c, out, _ := newTestCLI(t)
			c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
			if err := c.Run([]string{"restish", "completion", shell}); err != nil {
				t.Fatalf("completion %s: %v", shell, err)
			}
			if out.Len() == 0 {
				t.Errorf("completion %s: got empty output", shell)
			}
		})
	}
}

// TestSetupWritesAlias verifies that "setup zsh" appends the noglob alias to
// the shell rc file.
func TestSetupWritesAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "setup", "zsh", "--yes"}); err != nil {
		t.Fatalf("setup zsh: %v", err)
	}

	rcPath := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	if !strings.Contains(string(data), "noglob restish") {
		t.Errorf("expected noglob alias in zshrc, got: %q", string(data))
	}
	info, err := os.Stat(rcPath)
	if err != nil {
		t.Fatalf("stat zshrc: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected .zshrc to be created with 0600, got %#o", got)
	}
	if !strings.Contains(out.String(), ".zshrc") {
		t.Errorf("expected confirmation with rc path, got: %q", out.String())
	}
}

// TestSetupIdempotent verifies that running "setup" twice does not duplicate
// the alias.
func TestSetupIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for i := 0; i < 2; i++ {
		c, _, _ := newTestCLI(t)
		c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
		if err := c.Run([]string{"restish", "setup", "bash", "--yes"}); err != nil {
			t.Fatalf("run %d: setup bash: %v", i, err)
		}
	}

	rcName := ".bashrc"
	if runtime.GOOS == "darwin" {
		rcName = ".bash_profile"
	}
	rcPath := filepath.Join(home, rcName)
	data, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read %s: %v", rcName, err)
	}
	count := strings.Count(string(data), "noglob restish")
	if count != 1 {
		t.Errorf("expected alias to appear exactly once, got %d times:\n%s", count, string(data))
	}
}

// TestSetupUnsupportedShell verifies that an unknown shell returns an error.
func TestSetupUnsupportedShell(t *testing.T) {
	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "setup", "tcsh"})
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
}

func TestSetupFishSupported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "setup", "fish", "--yes"})
	if err != nil {
		t.Fatalf("expected fish setup to succeed, got: %v", err)
	}
	rcPath := filepath.Join(home, ".config", "fish", "config.fish")
	if _, statErr := os.Stat(rcPath); statErr != nil {
		t.Fatalf("expected fish config to be written: %v", statErr)
	}
}

func TestSetupWarnsForPermissiveExistingRCFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	rcName := ".bashrc"
	if runtime.GOOS == "darwin" {
		rcName = ".bash_profile"
	}
	rcPath := filepath.Join(home, rcName)
	if err := os.WriteFile(rcPath, []byte("# existing\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", rcName, err)
	}

	c, _, errOut := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "setup", "bash", "--yes"}); err != nil {
		t.Fatalf("setup bash: %v", err)
	}
	if !strings.Contains(errOut.String(), "chmod 600") {
		t.Fatalf("expected permission warning, got %q", errOut.String())
	}
}

func TestSetupDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "setup", "bash", "--dry-run"}); err != nil {
		t.Fatalf("setup bash dry-run: %v", err)
	}

	rcPath := filepath.Join(home, ".bashrc")
	if _, err := os.Stat(rcPath); !os.IsNotExist(err) {
		t.Fatalf("expected no file written in dry-run mode")
	}
	if !strings.Contains(out.String(), "Would update") {
		t.Fatalf("expected dry-run output, got: %q", out.String())
	}
}
