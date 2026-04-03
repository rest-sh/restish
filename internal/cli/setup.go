package cli

import (
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
	"fish": {rcFile: ".config/fish/config.fish", alias: `abbr --add restish "noglob restish"`},
}

// addSetupCommand registers the "setup" subcommand on root.
func (c *CLI) addSetupCommand(root *cobra.Command) {
	shells := make([]string, 0, len(shellSetups))
	for k := range shellSetups {
		shells = append(shells, k)
	}
	root.AddCommand(&cobra.Command{
		Use:       "setup <shell>",
		Short:     "Configure your shell for restish (writes a noglob alias)",
		Long:      fmt.Sprintf("Appends a noglob alias for restish to your shell rc file.\nSupported shells: %s", strings.Join(shells, ", ")),
		Args:      cobra.ExactArgs(1),
		ValidArgs: shells,
		RunE:      c.runSetup,
	})
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

	// Check if the alias is already present to avoid duplicates.
	existing, _ := os.ReadFile(rcPath)
	if strings.Contains(string(existing), setup.alias) {
		fmt.Fprintf(c.Stdout, "Shell already configured: %s already contains the restish alias.\n", rcPath)
		return nil
	}

	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("setup: cannot write %s: %w", rcPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("setup: write %s: %w", rcPath, err)
	}

	fmt.Fprintf(c.Stdout, "Configured %s: appended alias to %s\n", shell, rcPath)
	fmt.Fprintf(c.Stdout, "Restart your shell or run: source %s\n", rcPath)
	return nil
}
