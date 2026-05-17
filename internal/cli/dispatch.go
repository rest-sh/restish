package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func unknownSubcommandRun(command string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown %s command %q", command, args[0])
		}
		return cmd.Help()
	}
}

func rejectUnknownSubcommandHelp(root *cobra.Command, args []string) error {
	if !argsRequestHelp(args) {
		return nil
	}
	current := root
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return nil
		}
		if strings.HasPrefix(arg, "-") {
			if knownFlagConsumesNext(current, arg) {
				i++
				continue
			}
			if !knownHelpFlag(arg) && !knownFlag(current, arg) {
				return nil
			}
			continue
		}
		child := childCommandForToken(current, arg)
		if child == nil {
			if current != root && len(current.Commands()) > 0 {
				return unknownSubcommandError(current, arg)
			}
			return nil
		}
		current = child
	}
	return nil
}

func argsRequestHelp(args []string) bool {
	for _, arg := range args {
		if knownHelpFlag(arg) {
			return true
		}
	}
	return false
}

func knownHelpFlag(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "--help-all"
}

func knownFlagConsumesNext(cmd *cobra.Command, arg string) bool {
	flag := commandFlag(cmd, arg)
	return flag != nil && flag.NoOptDefVal == "" && !strings.Contains(arg, "=")
}

func knownFlag(cmd *cobra.Command, arg string) bool {
	return commandFlag(cmd, arg) != nil
}

func commandFlag(cmd *cobra.Command, arg string) *pflag.Flag {
	if cmd == nil || arg == "" || arg == "-" || arg == "--" {
		return nil
	}
	if strings.HasPrefix(arg, "--") {
		name := strings.TrimPrefix(arg, "--")
		if name, _, ok := strings.Cut(name, "="); ok {
			return cmd.Flag(name)
		}
		return cmd.Flag(name)
	}
	if strings.HasPrefix(arg, "-") {
		name := strings.TrimPrefix(arg, "-")
		if len(name) == 1 {
			return cmd.Flags().ShorthandLookup(name)
		}
	}
	return nil
}

func unknownSubcommandError(cmd *cobra.Command, arg string) error {
	switch cmd.Name() {
	case "api", "cache", "completion", "config", "plugin", "shell":
		return fmt.Errorf("unknown %s command %q", cmd.Name(), arg)
	default:
		if cmd.Annotations != nil {
			if cmd.Annotations[generatedAPIHelpShortAnnotation] != "" || cmd.Annotations[generatedAPIHelpFullAnnotation] != "" {
				return fmt.Errorf("unknown command %q for %q; run %q to see generated operations", arg, cmd.Name(), cmd.CommandPath()+" --help")
			}
		}
		return fmt.Errorf("unknown command %q for %q", arg, cmd.Name())
	}
}
