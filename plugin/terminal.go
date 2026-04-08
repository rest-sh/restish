package plugin

import "strings"

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
}

// TerminalContextFromArgs parses the Restish-injected terminal flags from the
// given argument slice (typically os.Args[1:]) and returns a TerminalContext.
// Unrecognised arguments are silently ignored.
//
//	ctx := plugin.TerminalContextFromArgs(os.Args[1:])
func TerminalContextFromArgs(args []string) TerminalContext {
	var ctx TerminalContext
	for _, arg := range args {
		if v, ok := strings.CutPrefix(arg, "--rsh-color="); ok {
			ctx.Color = v == "true"
		} else if v, ok := strings.CutPrefix(arg, "--rsh-stdout-tty="); ok {
			ctx.StdoutTTY = v == "true"
		} else if v, ok := strings.CutPrefix(arg, "--rsh-stderr-tty="); ok {
			ctx.StderrTTY = v == "true"
		}
	}
	return ctx
}
