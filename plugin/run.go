package plugin

import (
	"fmt"
	"io"
	"os"
)

// HandleStartupFlags checks os.Args for StartupFlagManifest or
// StartupFlagCommands. If either flag is found the appropriate CBOR
// response is written to w and the function returns true — the caller should
// return from main immediately.
//
//	func main() {
//	    if plugin.HandleStartupFlags(os.Stdout, manifest, cmds) { return }
//	    // ... command dispatch
//	}
func HandleStartupFlags(w io.Writer, m Manifest, cmds []CommandDecl) bool {
	for _, arg := range os.Args[1:] {
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
