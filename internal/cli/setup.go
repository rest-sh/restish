package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// shellSetup describes how to configure a shell for restish.
type shellSetup struct {
	// rcFile is the shell rc file relative to $HOME.
	rcFile string
	// alias is the line to append.
	alias string
}

var shellSetups = map[string]shellSetup{
	"zsh":  {rcFile: ".zshrc", alias: `alias restish="noglob restish"`},
	"bash": {rcFile: ".bashrc", alias: `alias restish="noglob restish"`},
	"fish": {rcFile: filepath.Join(".config", "fish", "config.fish"), alias: `function restish; command restish $argv; end`},
}

// addSetupCommand registers the "setup" subcommand on root.
func (c *CLI) addSetupCommand(root *cobra.Command) {
	shells := make([]string, 0, len(shellSetups))
	for k := range shellSetups {
		shells = append(shells, k)
	}
	setupCmd := &cobra.Command{
		Use:       "setup <shell>",
		Short:     "Configure your shell for restish (writes a noglob alias)",
		Long:      fmt.Sprintf("Appends a noglob alias for restish to your shell rc file.\nSupported shells: %s", strings.Join(shells, ", ")),
		Args:      cobra.ExactArgs(1),
		ValidArgs: shells,
		RunE:      c.runSetup,
	}
	setupCmd.Flags().Bool("dry-run", false, "Show what would be written without modifying files")
	setupCmd.Flags().BoolP("yes", "y", false, "Apply changes without confirmation prompt")
	root.AddCommand(setupCmd)
}

// runSetup appends a noglob alias to the appropriate shell rc file.
func (c *CLI) runSetup(cmd *cobra.Command, args []string) error {
	shell := strings.ToLower(args[0])
	setup, ok := shellSetups[shell]
	if !ok {
		return fmt.Errorf("unsupported shell %q; supported: zsh, bash, fish", shell)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("setup: cannot determine home directory: %w", err)
	}

	rcPath := filepath.Join(home, setup.rcFile)
	line := "\n" + setup.alias + "\n"
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	autoYes, _ := cmd.Flags().GetBool("yes")

	// Check if the alias is already present to avoid duplicates.
	existing, _ := os.ReadFile(rcPath)
	if strings.Contains(string(existing), setup.alias) {
		fmt.Fprintf(c.Stdout, "Shell already configured: %s already contains the restish alias.\n", rcPath)
		return nil
	}

	if dryRun {
		fmt.Fprintf(c.Stdout, "Would update %s with:\n%s\n", rcPath, setup.alias)
		return nil
	}

	if !autoYes {
		fmt.Fprintf(c.Stdout, "Update %s? [y/N]: ", rcPath)
		reader := bufio.NewReader(c.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(c.Stdout, "Cancelled.")
			return nil
		}
	}

	if info, err := os.Stat(rcPath); err == nil && info.Mode().Perm()&0o077 != 0 {
		fmt.Fprintf(c.Stderr, "warning: %s is more permissive than recommended; consider chmod 600 %s\n", rcPath, rcPath)
	}

	updated := string(existing) + line
	if err := atomicWriteTextFile(rcPath, []byte(updated), 0o600, 0o700); err != nil {
		return fmt.Errorf("setup: write %s: %w", rcPath, err)
	}

	fmt.Fprintf(c.Stdout, "Configured %s: appended alias to %s\n", shell, rcPath)
	fmt.Fprintf(c.Stdout, "Restart your shell or run: source %s\n", rcPath)
	return nil
}

func atomicWriteTextFile(path string, data []byte, fileMode, dirMode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(fileMode); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// hintShellSetup prints a first-run tip suggesting shell setup when the
// current shell is a supported one.  Called only when the config file is
// absent (true first run) and stderr is a TTY.
func (c *CLI) hintShellSetup() {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return
	}
	shell := strings.ToLower(filepath.Base(shellPath))
	if _, ok := shellSetups[shell]; !ok {
		return
	}
	fmt.Fprintf(c.Stderr, "tip: run `restish setup %s` to configure your shell (prevents glob expansion issues)\n", shell)
}
