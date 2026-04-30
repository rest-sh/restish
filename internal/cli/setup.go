package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var setupRuntimeGOOS = runtime.GOOS

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

// addShellCommand registers shell-integration commands on root.
func (c *CLI) addShellCommand(root *cobra.Command) {
	shells := make([]string, 0, len(shellSetups))
	for k := range shellSetups {
		shells = append(shells, k)
	}
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Configure shell integration for restish",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown shell command %q", args[0])
			}
			return cmd.Help()
		},
	}
	if rootCommandHasGroup(root, rootGroupConfig) {
		shellCmd.GroupID = rootGroupConfig
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
	setupCmd.Flags().Bool("completion", false, "Also install shell completion when supported")
	shellCmd.AddCommand(setupCmd)
	root.AddCommand(shellCmd)
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

	rcPath := setupRCPath(shell, home, setup)
	line := "\n" + setup.alias + "\n"
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	autoYes, _ := cmd.Flags().GetBool("yes")
	withCompletion, _ := cmd.Flags().GetBool("completion")
	if withCompletion && shell != "zsh" && shell != "fish" {
		return fmt.Errorf("setup: --completion is currently supported for zsh and fish")
	}

	// Check if the alias is already present to avoid duplicates.
	existing, _ := os.ReadFile(rcPath)
	aliasConfigured := strings.Contains(string(existing), setup.alias)
	if aliasConfigured && !withCompletion {
		fmt.Fprintf(c.Stdout, "Shell already configured: %s already contains the restish alias.\n", rcPath)
		return nil
	}

	if dryRun {
		if aliasConfigured {
			fmt.Fprintf(c.Stdout, "Shell already configured: %s already contains the restish alias.\n", rcPath)
		} else {
			fmt.Fprintf(c.Stdout, "Would update %s with:\n%s\n", rcPath, setup.alias)
		}
		if withCompletion {
			return c.installCompletion(cmd, completionInstallOptions{Shell: shell, DryRun: true})
		}
		return nil
	}

	if !autoYes && (!aliasConfigured || withCompletion) {
		if withCompletion {
			fmt.Fprintf(c.Stdout, "Update %s and install %s completion? [y/N]: ", rcPath, shell)
		} else {
			fmt.Fprintf(c.Stdout, "Update %s? [y/N]: ", rcPath)
		}
		ok, err := c.confirm()
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(c.Stdout, "Cancelled.")
			return nil
		}
	}

	if !aliasConfigured {
		if info, err := os.Stat(rcPath); err == nil && info.Mode().Perm()&0o077 != 0 {
			c.warnf("%s is more permissive than recommended; consider chmod 600 %s", rcPath, rcPath)
		}

		updated := string(existing) + line
		if err := atomicWriteTextFile(rcPath, []byte(updated), 0o600, 0o700); err != nil {
			return fmt.Errorf("setup: write %s: %w", rcPath, err)
		}

		fmt.Fprintf(c.Stdout, "Configured %s: appended alias to %s\n", shell, rcPath)
	} else {
		fmt.Fprintf(c.Stdout, "Shell already configured: %s already contains the restish alias.\n", rcPath)
	}

	if withCompletion {
		if err := c.installCompletion(cmd, completionInstallOptions{Shell: shell, Yes: true}); err != nil {
			return err
		}
	}

	fmt.Fprintf(c.Stdout, "Restart your shell or run: source %s\n", rcPath)
	return nil
}

func (c *CLI) confirm() (bool, error) {
	reader := bufio.NewReader(c.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && len(answer) == 0 {
		return false, nil
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes", nil
}

func setupRCPath(shell, home string, setup shellSetup) string {
	// macOS bash reads .bash_profile by default for login shells.
	if shell == "bash" && setupRuntimeGOOS == "darwin" {
		return filepath.Join(home, ".bash_profile")
	}
	if shell == "fish" {
		if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
			return filepath.Join(configHome, "fish", "config.fish")
		}
	}
	return filepath.Join(home, setup.rcFile)
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
	shell, source := detectRunningShell()
	if shell == "" {
		return
	}
	if _, ok := shellSetups[shell]; !ok {
		return
	}
	if source == "$SHELL" {
		c.tipf("run `restish shell setup %s` to configure your shell (prevents glob expansion issues; detected via $SHELL)", shell)
		return
	}
	c.tipf("run `restish shell setup %s` to configure your shell (prevents glob expansion issues)", shell)
}

func detectRunningShell() (string, string) {
	if setupRuntimeGOOS == "darwin" || setupRuntimeGOOS == "linux" {
		ppid := os.Getppid()
		if ppid > 1 {
			out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", ppid), "-o", "comm=").Output()
			if err == nil {
				name := strings.ToLower(filepath.Base(strings.TrimSpace(string(out))))
				if name != "" {
					return name, "ppid"
				}
			}
		}
	}

	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "", ""
	}
	return strings.ToLower(filepath.Base(shellPath)), "$SHELL"
}
