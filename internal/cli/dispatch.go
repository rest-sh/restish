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
			return unknownNamedSubcommandError(cmd, command, args[0], "")
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
		return unknownNamedSubcommandError(cmd, cmd.Name(), arg, "")
	default:
		if cmd.Annotations != nil {
			if cmd.Annotations[generatedAPIHelpShortAnnotation] != "" || cmd.Annotations[generatedAPIHelpFullAnnotation] != "" {
				return unknownCommandError(cmd, arg, "run "+strconvQuote(cmd.CommandPath()+" --help")+" to see generated operations")
			}
		}
		return unknownCommandError(cmd, arg, "")
	}
}

func unknownNamedSubcommandError(cmd *cobra.Command, group, arg, hint string) error {
	msg := fmt.Sprintf("unknown %s command %q", group, arg)
	if suggestion := commandSuggestionHint(cmd, arg); suggestion != "" {
		msg += "; " + suggestion
	}
	if hint != "" {
		msg += "; " + hint
	}
	return fmt.Errorf("%s", msg)
}

func unknownCommandError(cmd *cobra.Command, arg, hint string) error {
	msg := fmt.Sprintf("unknown command %q for %q", arg, cmd.CommandPath())
	if suggestion := commandSuggestionHint(cmd, arg); suggestion != "" {
		msg += "; " + suggestion
	}
	if hint != "" {
		msg += "; " + hint
	}
	return fmt.Errorf("%s", msg)
}

func commandSuggestionHint(cmd *cobra.Command, arg string) string {
	if cmd.SuggestionsMinimumDistance <= 0 {
		cmd.SuggestionsMinimumDistance = 2
	}
	suggestions := cmd.SuggestionsFor(arg)
	if len(suggestions) == 0 {
		if replacement := commandReplacementSuggestion(cmd, arg); replacement != "" {
			return fmt.Sprintf("did you mean %q?", replacement)
		}
		return ""
	}
	if len(suggestions) == 1 {
		return fmt.Sprintf("did you mean %q?", suggestions[0])
	}
	return fmt.Sprintf("did you mean one of %s?", strings.Join(quoteStrings(suggestions), ", "))
}

func commandReplacementSuggestion(cmd *cobra.Command, arg string) string {
	if cmd.Name() == "api" {
		switch arg {
		case "add", "configure":
			return "connect"
		case "delete":
			return "remove"
		}
	}
	argParts := strings.Split(arg, "-")
	if len(argParts) < 2 {
		return ""
	}
	var matches []string
	for _, sub := range cmd.Commands() {
		if sub.Hidden {
			continue
		}
		name := sub.Name()
		nameParts := strings.Split(name, "-")
		if len(nameParts) < 2 || nameParts[0] != argParts[0] {
			continue
		}
		if levenshteinDistance(arg, name) <= 7 {
			matches = append(matches, name)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	return ""
}

func levenshteinDistance(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i, ca := range ar {
		curr := make([]int, len(br)+1)
		curr[0] = i + 1
		for j, cb := range br {
			cost := 0
			if ca != cb {
				cost = 1
			}
			curr[j+1] = minInt(curr[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = curr
	}
	return prev[len(br)]
}

func minInt(values ...int) int {
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func quoteStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strconvQuote(value))
	}
	return out
}

func strconvQuote(value string) string {
	return fmt.Sprintf("%q", value)
}
