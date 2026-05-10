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
			if err := c.Run([]string{"restish", "shell", "completion", shell}); err != nil {
				t.Fatalf("shell completion %s: %v", shell, err)
			}
			if out.Len() == 0 {
				t.Errorf("completion %s: got empty output", shell)
			}
		})
	}
}

func TestCompletionInstallZshWritesScriptAndRCBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "completion", "install", "zsh", "--yes"}); err != nil {
		t.Fatalf("completion install zsh: %v", err)
	}

	scriptPath := filepath.Join(filepath.Dir(c.Hooks().ConfigPath), "completions", "_restish.zsh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read completion script: %v", err)
	}
	if !strings.Contains(string(script), "#compdef restish") {
		t.Fatalf("expected zsh completion script, got:\n%s", string(script))
	}

	rcPath := filepath.Join(home, ".zshrc")
	rc, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	rcText := string(rc)
	if !strings.Contains(rcText, "# >>> restish completion >>>") ||
		!strings.Contains(rcText, "compinit") ||
		!strings.Contains(rcText, "source '"+scriptPath+"'") {
		t.Fatalf("expected managed completion block in zshrc, got:\n%s", rcText)
	}
	if !strings.Contains(out.String(), "Installed zsh completion") {
		t.Fatalf("expected install confirmation, got: %q", out.String())
	}
	if count := strings.Count(out.String(), "Restart your shell or run: source "+rcPath); count != 1 {
		t.Fatalf("expected one restart instruction, got %d:\n%s", count, out.String())
	}
}

func TestCompletionInstallZshIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(t.TempDir(), "restish.json")

	for i := 0; i < 2; i++ {
		c, _, _ := newTestCLI(t)
		c.Hooks().ConfigPath = configPath
		if err := c.Run([]string{"restish", "completion", "install", "zsh", "--yes"}); err != nil {
			t.Fatalf("run %d: completion install zsh: %v", i, err)
		}
	}

	rcPath := filepath.Join(home, ".zshrc")
	rc, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	if count := strings.Count(string(rc), "# >>> restish completion >>>"); count != 1 {
		t.Fatalf("expected one managed completion block, got %d:\n%s", count, string(rc))
	}
}

func TestCompletionInstallZshDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "completion", "install", "zsh", "--dry-run"}); err != nil {
		t.Fatalf("completion install zsh --dry-run: %v", err)
	}

	scriptPath := filepath.Join(filepath.Dir(c.Hooks().ConfigPath), "completions", "_restish.zsh")
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Fatalf("expected no completion script written")
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("expected no zshrc written")
	}
	if !strings.Contains(out.String(), "Would write zsh completion script") {
		t.Fatalf("expected dry-run output, got: %q", out.String())
	}
}

func TestCompletionInstallZshDeclineReturnsError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	c.Hooks().PassReader = strings.NewReader("n\n")
	err := c.Run([]string{"restish", "completion", "install", "zsh"})
	if err == nil {
		t.Fatal("expected declined completion install to return an error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Cancelled.") {
		t.Fatalf("expected cancellation output, got:\n%s", out.String())
	}
}

func TestCompletionInstallFishWritesScript(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "completion", "install", "fish", "--yes"}); err != nil {
		t.Fatalf("completion install fish: %v", err)
	}

	scriptPath := filepath.Join(configHome, "fish", "completions", "restish.fish")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read fish completion script: %v", err)
	}
	if !strings.Contains(string(script), "complete -c restish") {
		t.Fatalf("expected fish completion script, got:\n%s", string(script))
	}
	if !strings.Contains(out.String(), "Installed fish completion") {
		t.Fatalf("expected install confirmation, got: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("expected fish install not to write zshrc")
	}
}

func TestCompletionInstallFishIdempotent(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	c, _, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "completion", "install", "fish", "--yes"}); err != nil {
		t.Fatalf("completion install fish: %v", err)
	}
	first, err := os.ReadFile(filepath.Join(configHome, "fish", "completions", "restish.fish"))
	if err != nil {
		t.Fatalf("read fish completion script: %v", err)
	}

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "completion", "install", "fish", "--yes"}); err != nil {
		t.Fatalf("second completion install fish: %v", err)
	}
	second, err := os.ReadFile(filepath.Join(configHome, "fish", "completions", "restish.fish"))
	if err != nil {
		t.Fatalf("read fish completion script after second run: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("expected fish completion script to remain unchanged")
	}
	if !strings.Contains(out.String(), "Fish completion already installed") {
		t.Fatalf("expected idempotent confirmation, got: %q", out.String())
	}
}

func TestCompletionInstallFishDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "completion", "install", "fish", "--dry-run"}); err != nil {
		t.Fatalf("completion install fish --dry-run: %v", err)
	}

	scriptPath := filepath.Join(configHome, "fish", "completions", "restish.fish")
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Fatalf("expected no fish completion script written")
	}
	if !strings.Contains(out.String(), "Would write fish completion script") {
		t.Fatalf("expected dry-run output, got: %q", out.String())
	}
}

func TestCompletionInstallFishDeclineReturnsError(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	c, out, _ := newTestCLI(t)
	c.Hooks().PassReader = strings.NewReader("n\n")
	err := c.Run([]string{"restish", "completion", "install", "fish"})
	if err == nil {
		t.Fatal("expected declined completion install to return an error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Cancelled.") {
		t.Fatalf("expected cancellation output, got:\n%s", out.String())
	}
}

// TestSetupWritesAlias verifies that "setup zsh" appends the noglob alias to
// the shell rc file.
func TestSetupWritesAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	if err := c.Run([]string{"restish", "shell", "setup", "zsh", "--yes"}); err != nil {
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

func TestSetupZshInstallsCompletionByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "shell", "setup", "zsh", "--yes"}); err != nil {
		t.Fatalf("setup zsh: %v", err)
	}

	rcPath := filepath.Join(home, ".zshrc")
	rc, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	rcText := string(rc)
	if !strings.Contains(rcText, `alias restish="noglob restish"`) {
		t.Fatalf("expected noglob alias, got:\n%s", rcText)
	}
	if !strings.Contains(rcText, "# >>> restish completion >>>") {
		t.Fatalf("expected completion block, got:\n%s", rcText)
	}

	scriptPath := filepath.Join(filepath.Dir(c.Hooks().ConfigPath), "completions", "_restish.zsh")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("expected completion script: %v", err)
	}
	if !strings.Contains(out.String(), "Installed zsh completion") {
		t.Fatalf("expected completion install confirmation, got: %q", out.String())
	}
	if count := strings.Count(out.String(), "Restart your shell or run: source "+rcPath); count != 1 {
		t.Fatalf("expected one restart instruction, got %d:\n%s", count, out.String())
	}
}

func TestSetupDeclineReturnsError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, out, _ := newTestCLI(t)
	c.Hooks().PassReader = strings.NewReader("n\n")
	err := c.Run([]string{"restish", "shell", "setup", "zsh"})
	if err == nil {
		t.Fatal("expected declined setup to return an error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Cancelled.") {
		t.Fatalf("expected cancellation output, got:\n%s", out.String())
	}
	if _, statErr := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no zshrc after cancelled setup, stat err = %v", statErr)
	}
}

func TestSetupEOFReturnsError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, _, _ := newTestCLI(t)
	err := c.Run([]string{"restish", "shell", "setup", "zsh"})
	if err == nil {
		t.Fatal("expected noninteractive setup to return an error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetupZshNoCompletion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, _, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "shell", "setup", "zsh", "--no-completion", "--yes"}); err != nil {
		t.Fatalf("setup zsh --no-completion: %v", err)
	}

	rcPath := filepath.Join(home, ".zshrc")
	rc, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read zshrc: %v", err)
	}
	if strings.Contains(string(rc), "# >>> restish completion >>>") {
		t.Fatalf("did not expect completion block, got:\n%s", string(rc))
	}
	scriptPath := filepath.Join(filepath.Dir(c.Hooks().ConfigPath), "completions", "_restish.zsh")
	if _, err := os.Stat(scriptPath); !os.IsNotExist(err) {
		t.Fatalf("expected no completion script, stat err=%v", err)
	}
}

func TestSetupFishInstallsCompletionByDefault(t *testing.T) {
	home := t.TempDir()
	configHome := filepath.Join(home, ".config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	c, out, _ := newTestCLI(t)
	if err := c.Run([]string{"restish", "shell", "setup", "fish", "--yes"}); err != nil {
		t.Fatalf("setup fish: %v", err)
	}

	rcPath := filepath.Join(configHome, "fish", "config.fish")
	rc, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("read fish config: %v", err)
	}
	if !strings.Contains(string(rc), "function restish; command restish $argv; end") {
		t.Fatalf("expected fish wrapper, got:\n%s", string(rc))
	}

	scriptPath := filepath.Join(configHome, "fish", "completions", "restish.fish")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("expected fish completion script: %v", err)
	}
	if !strings.Contains(out.String(), "Installed fish completion") {
		t.Fatalf("expected fish completion install confirmation, got: %q", out.String())
	}
}

// TestSetupIdempotent verifies that running "shell", "setup" twice does not duplicate
// the alias.
func TestSetupIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for i := 0; i < 2; i++ {
		c, _, _ := newTestCLI(t)
		c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
		if err := c.Run([]string{"restish", "shell", "setup", "bash", "--yes"}); err != nil {
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
	err := c.Run([]string{"restish", "shell", "setup", "tcsh"})
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
}

func TestSetupFishSupported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	c, _, _ := newTestCLI(t)
	c.Hooks().ConfigPath = t.TempDir() + "/restish.json"
	err := c.Run([]string{"restish", "shell", "setup", "fish", "--yes"})
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
	if err := c.Run([]string{"restish", "shell", "setup", "bash", "--yes"}); err != nil {
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
	if err := c.Run([]string{"restish", "shell", "setup", "bash", "--dry-run"}); err != nil {
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
