package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSetupRCPath_DarwinBashProfile(t *testing.T) {
	oldGOOS := setupRuntimeGOOS
	setupRuntimeGOOS = "darwin"
	defer func() { setupRuntimeGOOS = oldGOOS }()

	got := setupRCPath("bash", "/home/alice", shellSetups["bash"])
	want := filepath.Join("/home/alice", ".bash_profile")
	if got != want {
		t.Fatalf("setupRCPath() = %q, want %q", got, want)
	}
}

func TestDetectRunningShell_FallbackToEnv(t *testing.T) {
	oldGOOS := setupRuntimeGOOS
	setupRuntimeGOOS = "unknown"
	defer func() { setupRuntimeGOOS = oldGOOS }()

	t.Setenv("SHELL", "/bin/zsh")
	shell, source := detectRunningShell()
	if shell != "zsh" || source != "$SHELL" {
		t.Fatalf("detectRunningShell() = (%q, %q), want (%q, %q)", shell, source, "zsh", "$SHELL")
	}
}

func TestHintShellSetup_FallbackNote(t *testing.T) {
	oldGOOS := setupRuntimeGOOS
	setupRuntimeGOOS = "unknown"
	defer func() { setupRuntimeGOOS = oldGOOS }()

	t.Setenv("SHELL", "/bin/bash")
	c := New()
	var stderr bytes.Buffer
	c.Stderr = &stderr
	c.hintShellSetup()
	if !strings.Contains(stderr.String(), "detected via $SHELL") {
		t.Fatalf("expected fallback note in hint output, got %q", stderr.String())
	}
}

func TestRunSetup_DarwinBashWritesProfile(t *testing.T) {
	oldGOOS := setupRuntimeGOOS
	setupRuntimeGOOS = "darwin"
	defer func() { setupRuntimeGOOS = oldGOOS }()

	home := t.TempDir()
	t.Setenv("HOME", home)
	c := New()
	c.Stdin = strings.NewReader("yes\n")
	c.Stdout = &bytes.Buffer{}
	c.Stderr = &bytes.Buffer{}

	root := &cobra.Command{Use: "restish"}
	c.addSetupCommand(root)
	root.SetArgs([]string{"setup", "bash", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute setup bash: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".bash_profile")); err != nil {
		t.Fatalf("expected .bash_profile to be written: %v", err)
	}
}
