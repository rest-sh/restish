package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rest-sh/restish/v2/internal/fileutil"
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
		Long:  shellLong,
		Example: fmt.Sprintf(`  %s shell setup zsh
  %s shell completion zsh
  %s shell completion install zsh`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
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
		Use:   "setup <shell>",
		Short: "Configure your shell for restish",
		Long:  shellSetupLong,
		Example: fmt.Sprintf(`  %s shell setup zsh
  %s shell setup fish --dry-run
  %s shell setup bash --no-completion`, c.commandNameOrDefault(), c.commandNameOrDefault(), c.commandNameOrDefault()),
		Args:      usageExactArgs(1),
		ValidArgs: shells,
		RunE:      c.runSetup,
	}
	setupCmd.Flags().Bool("dry-run", false, "Show what would be written without modifying files")
	setupCmd.Flags().BoolP("yes", "y", false, "Apply changes without confirmation prompt")
	setupCmd.Flags().Bool("no-completion", false, "Do not install shell completion")
	setupCmd.Flags().Bool("completion", false, "Install shell completion when supported")
	_ = setupCmd.Flags().MarkHidden("completion")
	completionCmd := c.newCompletionCommand(root)
	completionCmd.Short = "Generate shell completion scripts"
	completionCmd.GroupID = ""
	shellCmd.AddCommand(setupCmd, completionCmd)
	root.AddCommand(shellCmd)
}

// runSetup appends a noglob alias to the appropriate shell rc file.
func (c *CLI) runSetup(cmd *cobra.Command, args []string) error {
	shell := strings.ToLower(args[0])
	setup, ok := shellSetups[shell]
	if !ok {
		return newUsageError(fmt.Errorf("unsupported shell %q; supported: zsh, bash, fish", shell))
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("setup: cannot determine home directory: %w", err)
	}

	rcPath := setupRCPath(shell, home, setup)
	line := "\n" + setup.alias + "\n"
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	autoYes, _ := cmd.Flags().GetBool("yes")
	noCompletion, _ := cmd.Flags().GetBool("no-completion")
	completionFlag, _ := cmd.Flags().GetBool("completion")
	withCompletion := !noCompletion && (shell == "zsh" || shell == "fish")
	if completionFlag {
		withCompletion = shell == "zsh" || shell == "fish"
	}
	if withCompletion && shell != "zsh" && shell != "fish" {
		return fmt.Errorf("setup: --completion is currently supported for zsh and fish")
	}

	// Check if the alias is already present to avoid duplicates.
	existing, _ := os.ReadFile(rcPath)
	aliasConfigured := strings.Contains(string(existing), setup.alias)
	style := humanTextStyleFor(c.Stdout)
	if aliasConfigured && !withCompletion {
		fmt.Fprintf(c.Stdout, "Shell %s: %s already contains the restish alias.\n", style.ok("already configured"), rcPath)
		return nil
	}

	if dryRun {
		if aliasConfigured {
			fmt.Fprintf(c.Stdout, "Shell %s: %s already contains the restish alias.\n", style.ok("already configured"), rcPath)
		} else {
			fmt.Fprintf(c.Stdout, "%s %s with:\n%s\n", style.hint("Would update"), rcPath, setup.alias)
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
		ok, err := c.confirm(cmd.Context())
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintf(c.Stdout, "%s.\n", style.warn("Cancelled"))
			return fmt.Errorf("setup: cancelled")
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

		fmt.Fprintf(c.Stdout, "%s %s: appended alias to %s\n", style.ok("Configured"), shell, rcPath)
	} else {
		fmt.Fprintf(c.Stdout, "Shell %s: %s already contains the restish alias.\n", style.ok("already configured"), rcPath)
	}

	if withCompletion {
		if err := c.installCompletion(cmd, completionInstallOptions{Shell: shell, Yes: true, SuppressRestartHint: shell == "zsh"}); err != nil {
			return err
		}
	}

	fmt.Fprintf(c.Stdout, "%s source %s\n", style.hint("Restart your shell or run:"), rcPath)
	return nil
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
	return fileutil.AtomicWriteFile(path, data, fileutil.AtomicWriteOptions{
		FileMode: fileMode,
		DirMode:  dirMode,
	})
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
				name := normalizeShellName(strings.TrimSpace(string(out)))
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
	return normalizeShellName(shellPath), "$SHELL"
}

func normalizeShellName(name string) string {
	name = strings.ToLower(filepath.Base(strings.TrimSpace(name)))
	return strings.TrimLeft(name, "-")
}
