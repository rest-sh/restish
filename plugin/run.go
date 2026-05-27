package plugin

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// HandleStartupFlags checks the Restish-injected startup prefix in os.Args for
// StartupFlagManifest or StartupFlagCommands. If either flag is found the
// appropriate CBOR response is written to w and the function returns true —
// the caller should return from main immediately.
//
//	func main() {
//	    if plugin.HandleStartupFlags(os.Stdout, manifest, cmds) { return }
//	    // ... command dispatch
//	}
func HandleStartupFlags(w io.Writer, m Manifest, cmds []CommandDecl) bool {
	args := os.Args[1:]
	for _, arg := range args[:startupPrefixEnd(args)] {
		switch arg {
		case StartupFlagManifest:
			if err := WriteManifest(w, m); err != nil {
				fmt.Fprintln(os.Stderr, "manifest:", err)
				os.Exit(2)
			}
			return true
		case StartupFlagCommands:
			if err := WriteCommands(w, cmds); err != nil {
				fmt.Fprintln(os.Stderr, "commands:", err)
				os.Exit(2)
			}
			return true
		}
	}
	return false
}

// ArgsWithoutStartupFlags strips Restish-injected startup and terminal flags
// from the beginning of args. Flags with the same names after the first user
// argument are preserved as user input.
func ArgsWithoutStartupFlags(args []string) []string {
	return args[startupPrefixEnd(args):]
}

func startupPrefixEnd(args []string) int {
	for i, arg := range args {
		if !isStartupPrefixArg(arg) {
			return i
		}
	}
	return len(args)
}

func isStartupPrefixArg(arg string) bool {
	switch arg {
	case StartupFlagManifest, StartupFlagCommands:
		return true
	}
	return strings.HasPrefix(arg, StartupFlagColor+"=") ||
		strings.HasPrefix(arg, StartupFlagStdoutTTY+"=") ||
		strings.HasPrefix(arg, StartupFlagStderrTTY+"=") ||
		strings.HasPrefix(arg, StartupFlagTheme+"=")
}

// Run is the complete main-loop for simple command plugins. It handles
// startup flags, reads the init message from os.Stdin, creates a
// CommandClient, and calls fn with the command name, args, and client.
//
// On success Run sends a done message with exit code 0 and returns.
// On error Run sends the error to stderr and exits with code 1.
//
//	func main() {
//	    plugin.Run(manifest, cmds, func(command string, args []string, c *plugin.CommandClient) error {
//	        // ... handle command
//	        return nil
//	    })
//	}
func Run(m Manifest, cmds []CommandDecl, fn func(command string, args []string, client *CommandClient) error) {
	if HandleStartupFlags(os.Stdout, m, cmds) {
		return
	}

	var initMsg InitMsg
	client := NewCommandClient(os.Stdin, os.Stdout)
	if err := client.ReadMessage(&initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}

	command := initMsg.Command
	args := initMsg.Args

	if err := fn(command, args, client); err != nil {
		if writeErr := client.WriteStderr([]byte(err.Error() + "\n")); writeErr != nil {
			fmt.Fprintln(os.Stderr, "write stderr:", writeErr)
			os.Exit(1)
		}
		if doneErr := client.Done(1); doneErr != nil {
			fmt.Fprintln(os.Stderr, "write done:", doneErr)
			os.Exit(1)
		}
		return
	}
	if err := client.Done(0); err != nil {
		fmt.Fprintln(os.Stderr, "write done:", err)
		os.Exit(1)
	}
}
