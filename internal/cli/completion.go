package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	completionBlockStart = "# >>> restish completion >>>"
	completionBlockEnd   = "# <<< restish completion <<<"
)

type completionInstallOptions struct {
	Shell  string
	DryRun bool
	Yes    bool
	NoDesc bool
}

func (c *CLI) addCompletionCommand(root *cobra.Command) {
	var noDesc bool
	completionCmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate or install shell completion scripts",
		Long: `Generate shell completion scripts, or install completion for your user account.

Script generation writes to stdout for package managers and manual setup.
The install command writes a generated script under Restish's config directory
or a shell-native user completion directory, then updates shell startup files
only when the shell requires it.`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	if rootCommandHasGroup(root, rootGroupHelp) {
		completionCmd.GroupID = rootGroupHelp
	}

	shortDesc := "Generate the autocompletion script for %s"
	bash := &cobra.Command{
		Use:                   "bash",
		Short:                 fmt.Sprintf(shortDesc, "bash"),
		Long:                  shellCompletionLong("bash", "restish completion bash > /etc/bash_completion.d/restish"),
		Args:                  cobra.NoArgs,
		DisableFlagsInUseLine: true,
		ValidArgsFunction:     cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletionScript(cmd.Root(), "bash", noDesc, c.Stdout)
		},
	}
	zsh := &cobra.Command{
		Use:               "zsh",
		Short:             fmt.Sprintf(shortDesc, "zsh"),
		Long:              shellCompletionLong("zsh", "restish completion install zsh"),
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletionScript(cmd.Root(), "zsh", noDesc, c.Stdout)
		},
	}
	fish := &cobra.Command{
		Use:               "fish",
		Short:             fmt.Sprintf(shortDesc, "fish"),
		Long:              shellCompletionLong("fish", "restish completion install fish"),
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletionScript(cmd.Root(), "fish", noDesc, c.Stdout)
		},
	}
	powershell := &cobra.Command{
		Use:               "powershell",
		Short:             fmt.Sprintf(shortDesc, "powershell"),
		Long:              shellCompletionLong("powershell", "restish completion powershell | Out-String | Invoke-Expression"),
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletionScript(cmd.Root(), "powershell", noDesc, c.Stdout)
		},
	}
	for _, cmd := range []*cobra.Command{bash, zsh, fish, powershell} {
		cmd.Flags().BoolVar(&noDesc, "no-descriptions", false, "disable completion descriptions")
	}

	installCmd := &cobra.Command{
		Use:               "install <shell>",
		Short:             "Install shell completion for your user account",
		Long:              "Install shell completion for your user account.\n\nSupported shells: zsh, fish. The zsh installer writes the generated script under Restish's config directory and adds a managed source block to ~/.zshrc. The fish installer writes to the shell's user completions directory.",
		Args:              cobra.ExactArgs(1),
		ValidArgs:         []string{"zsh", "fish"},
		ValidArgsFunction: cobra.FixedCompletions([]cobra.Completion{"zsh", "fish"}, cobra.ShellCompDirectiveNoFileComp),
		RunE: func(cmd *cobra.Command, args []string) error {
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			yes, _ := cmd.Flags().GetBool("yes")
			noDesc, _ := cmd.Flags().GetBool("no-descriptions")
			return c.installCompletion(cmd, completionInstallOptions{
				Shell:  strings.ToLower(args[0]),
				DryRun: dryRun,
				Yes:    yes,
				NoDesc: noDesc,
			})
		},
	}
	installCmd.Flags().Bool("dry-run", false, "Show what would be written without modifying files")
	installCmd.Flags().BoolP("yes", "y", false, "Apply changes without confirmation prompt")
	installCmd.Flags().Bool("no-descriptions", false, "disable completion descriptions")

	completionCmd.AddCommand(bash, zsh, fish, powershell, installCmd)
	root.AddCommand(completionCmd)
}

func shellCompletionLong(shell, installExample string) string {
	return fmt.Sprintf(`Generate the autocompletion script for %s.

This writes the script to stdout for package managers and manual shell setup.
For user-level installation, use:

  %s
`, shell, installExample)
}

func generateCompletionScript(root *cobra.Command, shell string, noDesc bool, out io.Writer) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(out, !noDesc)
	case "zsh":
		if noDesc {
			return root.GenZshCompletionNoDesc(out)
		}
		return root.GenZshCompletion(out)
	case "fish":
		return root.GenFishCompletion(out, !noDesc)
	case "powershell":
		if noDesc {
			return root.GenPowerShellCompletion(out)
		}
		return root.GenPowerShellCompletionWithDesc(out)
	default:
		return fmt.Errorf("unsupported shell %q; supported: bash, zsh, fish, powershell", shell)
	}
}

func (c *CLI) installCompletion(cmd *cobra.Command, opts completionInstallOptions) error {
	switch opts.Shell {
	case "zsh":
		return c.installZshCompletion(cmd, opts)
	case "fish":
		return c.installFishCompletion(cmd, opts)
	default:
		return fmt.Errorf("completion install: unsupported shell %q; supported: zsh, fish", opts.Shell)
	}
}

func (c *CLI) installZshCompletion(cmd *cobra.Command, opts completionInstallOptions) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("completion install: cannot determine home directory: %w", err)
	}

	scriptPath := c.completionScriptPath(cmd.Root().Name(), opts.Shell)
	rcPath := filepath.Join(home, ".zshrc")

	var script bytes.Buffer
	if err := generateCompletionScript(cmd.Root(), opts.Shell, opts.NoDesc, &script); err != nil {
		return err
	}
	rcBlock := zshCompletionRCBlock(scriptPath)

	existingRCBytes, _ := os.ReadFile(rcPath)
	existingRC := string(existingRCBytes)
	updatedRC, rcChanged := upsertManagedBlock(existingRC, completionBlockStart, completionBlockEnd, rcBlock)

	existingScript, _ := os.ReadFile(scriptPath)
	scriptChanged := !bytes.Equal(existingScript, script.Bytes())

	if !scriptChanged && !rcChanged {
		fmt.Fprintf(c.Stdout, "Zsh completion already installed: %s\n", scriptPath)
		return nil
	}

	if opts.DryRun {
		fmt.Fprintf(c.Stdout, "Would write zsh completion script to %s\n", scriptPath)
		if rcChanged {
			fmt.Fprintf(c.Stdout, "Would update %s with:\n%s\n", rcPath, rcBlock)
		}
		return nil
	}

	if !opts.Yes {
		fmt.Fprintf(c.Stdout, "Install zsh completion and update %s? [y/N]: ", rcPath)
		ok, err := c.confirm(cmd.Context())
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(c.Stdout, "Cancelled.")
			return nil
		}
	}

	if scriptChanged {
		if err := atomicWriteTextFile(scriptPath, script.Bytes(), 0o600, 0o700); err != nil {
			return fmt.Errorf("completion install: write %s: %w", scriptPath, err)
		}
	}
	if rcChanged {
		if info, err := os.Stat(rcPath); err == nil && info.Mode().Perm()&0o077 != 0 {
			c.warnf("%s is more permissive than recommended; consider chmod 600 %s", rcPath, rcPath)
		}
		if err := atomicWriteTextFile(rcPath, []byte(updatedRC), 0o600, 0o700); err != nil {
			return fmt.Errorf("completion install: write %s: %w", rcPath, err)
		}
	}

	fmt.Fprintf(c.Stdout, "Installed zsh completion: %s\n", scriptPath)
	if rcChanged {
		fmt.Fprintf(c.Stdout, "Updated %s\n", rcPath)
	}
	fmt.Fprintf(c.Stdout, "Restart your shell or run: source %s\n", rcPath)
	return nil
}

func (c *CLI) installFishCompletion(cmd *cobra.Command, opts completionInstallOptions) error {
	scriptPath, err := fishCompletionScriptPath(cmd.Root().Name())
	if err != nil {
		return err
	}

	var script bytes.Buffer
	if err := generateCompletionScript(cmd.Root(), opts.Shell, opts.NoDesc, &script); err != nil {
		return err
	}

	existingScript, _ := os.ReadFile(scriptPath)
	scriptChanged := !bytes.Equal(existingScript, script.Bytes())
	if !scriptChanged {
		fmt.Fprintf(c.Stdout, "Fish completion already installed: %s\n", scriptPath)
		return nil
	}

	if opts.DryRun {
		fmt.Fprintf(c.Stdout, "Would write fish completion script to %s\n", scriptPath)
		return nil
	}

	if !opts.Yes {
		fmt.Fprintf(c.Stdout, "Install fish completion to %s? [y/N]: ", scriptPath)
		ok, err := c.confirm(cmd.Context())
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(c.Stdout, "Cancelled.")
			return nil
		}
	}

	if err := atomicWriteTextFile(scriptPath, script.Bytes(), 0o600, 0o700); err != nil {
		return fmt.Errorf("completion install: write %s: %w", scriptPath, err)
	}

	fmt.Fprintf(c.Stdout, "Installed fish completion: %s\n", scriptPath)
	fmt.Fprintln(c.Stdout, "Start a new fish session for completion to take effect.")
	return nil
}

func (c *CLI) completionScriptPath(commandName, shell string) string {
	return filepath.Join(filepath.Dir(c.configFilePath()), "completions", completionScriptFilename(commandName, shell))
}

func fishCompletionScriptPath(commandName string) (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("completion install: cannot determine home directory: %w", err)
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "fish", "completions", sanitizePathComponent(commandName)+".fish"), nil
}

func completionScriptFilename(commandName, shell string) string {
	name := sanitizePathComponent(commandName)
	switch shell {
	case "zsh":
		return "_" + name + ".zsh"
	default:
		return name + "." + shell
	}
}

func sanitizePathComponent(value string) string {
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "restish"
	}
	return b.String()
}

func zshCompletionRCBlock(scriptPath string) string {
	quotedPath := shellSingleQuote(scriptPath)
	return strings.Join([]string{
		completionBlockStart,
		"# Managed by `restish completion install zsh`.",
		"autoload -Uz compinit",
		"if ! whence -w compdef >/dev/null 2>&1; then",
		"  compinit",
		"fi",
		"if [ -r " + quotedPath + " ]; then",
		"  source " + quotedPath,
		"fi",
		completionBlockEnd,
		"",
	}, "\n")
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func upsertManagedBlock(existing, start, end, block string) (string, bool) {
	startIdx := strings.Index(existing, start)
	if startIdx >= 0 {
		endIdx := strings.Index(existing[startIdx:], end)
		if endIdx >= 0 {
			endIdx += startIdx + len(end)
			for endIdx < len(existing) && (existing[endIdx] == '\r' || existing[endIdx] == '\n') {
				endIdx++
			}
			updated := existing[:startIdx] + block + existing[endIdx:]
			return updated, updated != existing
		}
	}

	prefix := existing
	if prefix != "" && !strings.HasSuffix(prefix, "\n") {
		prefix += "\n"
	}
	updated := prefix + block
	return updated, updated != existing
}
