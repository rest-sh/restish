// cmdplugin is a test Restish command plugin used in command-plugin tests.
package main

import (
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/plugin"
)

var manifest = plugin.Manifest{
	Name:              "cmdplugin",
	Version:           "1.0.0",
	Description:       "Test command plugin",
	RestishAPIVersion: 2,
	Hooks:             []string{"command"},
}

var commands = []plugin.CommandDecl{
	{Name: "greet", Short: "Greet the user"},
	{Name: "fetch", Short: "Fetch a URL via Restish"},
	{Name: "pipe", Short: "Echo stdin via passthrough stdio", PassthroughStdio: true},
	{Name: "fail", Short: "Exit with code 1"},
	{Name: "die", Short: "Crash unexpectedly"},
}

func main() {
	if plugin.HandleStartupFlags(os.Stdout, manifest, commands) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)

	var initMsg plugin.InitMsg
	if err := dec.ReadMessage(&initMsg); err != nil {
		fmt.Fprintln(os.Stderr, "read init:", err)
		os.Exit(1)
	}

	switch initMsg.Command {
	case "greet":
		_ = plugin.WriteMessage(os.Stdout, plugin.WarnMsg{Type: plugin.MsgTypeWarn, Text: "Greeting in progress..."})
		_ = plugin.WriteMessage(os.Stdout, plugin.ResponseMsg{
			Type:   plugin.MsgTypeResponse,
			Status: 200,
			Body:   map[string]any{"greeting": "Hello from plugin!"},
		})
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone})

	case "fetch":
		var fetchURL string
		if len(initMsg.Args) > 0 {
			fetchURL = initMsg.Args[0]
		}
		if fetchURL == "" {
			_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone, ExitCode: 1})
			return
		}
		_ = plugin.WriteMessage(os.Stdout, plugin.HTTPRequestMsg{
			Type:   plugin.MsgTypeHTTPRequest,
			Method: "GET",
			URI:    fetchURL,
		})
		var httpResp plugin.HTTPResponseMsg
		if err := dec.ReadMessage(&httpResp); err != nil {
			fmt.Fprintln(os.Stderr, "read http-response:", err)
			os.Exit(1)
		}
		_ = plugin.WriteMessage(os.Stdout, plugin.ResponseMsg{
			Type:   plugin.MsgTypeResponse,
			Status: httpResp.Status,
			Body:   httpResp.Body,
		})
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone})

	case "fail":
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone, ExitCode: 1})

	case "pipe":
		for {
			raw, err := dec.ReadRaw()
			if err != nil {
				os.Exit(1)
			}
			switch plugin.MessageType(raw) {
			case plugin.MsgTypeStdinData:
				var msg plugin.StdinDataMsg
				_ = plugin.DecMode.Unmarshal(raw, &msg)
				_ = plugin.WriteMessage(os.Stdout, plugin.StdoutDataMsg{
					Type: plugin.MsgTypeStdoutData,
					Data: append([]byte("OUT:"), msg.Data...),
				})
				_ = plugin.WriteMessage(os.Stdout, plugin.StderrDataMsg{
					Type: plugin.MsgTypeStderrData,
					Data: append([]byte("ERR:"), msg.Data...),
				})
			case plugin.MsgTypeStdinClose:
				_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone})
				return
			}
		}

	case "die":
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", initMsg.Command)
		_ = plugin.WriteMessage(os.Stdout, plugin.DoneMsg{Type: plugin.MsgTypeDone, ExitCode: 1})
	}
}
