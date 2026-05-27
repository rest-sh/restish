package plugin

import (
	"encoding/json"
	"strings"
)

// TerminalContext carries terminal capability flags that the Restish host
// passes to command plugins as CLI arguments.  Plugins that care about colour
// or TTY detection should call TerminalContextFromArgs(os.Args[1:]).
type TerminalContext struct {
	// Color is true when the host terminal supports ANSI colour sequences.
	Color bool
	// StdoutTTY is true when the host's stdout is connected to a terminal.
	StdoutTTY bool
	// StderrTTY is true when the host's stderr is connected to a terminal.
	StderrTTY bool
	// Theme is the host's configured Restish terminal theme entries.
	Theme map[string]string
}

// TerminalContextFromArgs parses the Restish-injected terminal flags from the
// given argument slice (typically os.Args[1:]) and returns a TerminalContext.
// Unrecognised arguments are silently ignored.
//
//	ctx := plugin.TerminalContextFromArgs(os.Args[1:])
func TerminalContextFromArgs(args []string) TerminalContext {
	var ctx TerminalContext
	for _, arg := range args[:startupPrefixEnd(args)] {
		if v, ok := strings.CutPrefix(arg, StartupFlagColor+"="); ok {
			ctx.Color = v == "true"
		} else if v, ok := strings.CutPrefix(arg, StartupFlagStdoutTTY+"="); ok {
			ctx.StdoutTTY = v == "true"
		} else if v, ok := strings.CutPrefix(arg, StartupFlagStderrTTY+"="); ok {
			ctx.StderrTTY = v == "true"
		} else if v, ok := strings.CutPrefix(arg, StartupFlagTheme+"="); ok {
			var theme map[string]string
			if err := json.Unmarshal([]byte(v), &theme); err == nil {
				ctx.Theme = theme
			}
		}
	}
	return ctx
}
