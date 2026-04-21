---
title: Plugin Quickstart
linkTitle: Quickstart
weight: 10
description: Build your first Restish plugin and choose the right plugin type.
---

This guide is the shortest path from "I want to extend Restish" to a working
plugin binary.

## Path

`Documentation -> Plugins -> Quickstart`

If you are trying to use an existing plugin rather than write one, start with:

- [Plugins Reference](/docs/reference/plugins/)
- [Bulk Management](/docs/guides/bulk-management/)
- [MCP](/docs/guides/mcp/)

## Choose a Plugin Type

- Hook plugin: one request in, one reply out. Best for auth, request
  middleware, response middleware, custom spec loaders, and output formatters.
- Command plugin: a long-lived top-level command such as `restish mcp ...` that
  can delegate HTTP calls back to the host.
- TLS signer plugin: advanced mutual TLS use cases where the private key must
  stay outside the Restish process.

Start with a hook plugin unless you already know you need a custom command
lifecycle.

In practice:

- start with a formatter hook plugin if you want the fastest first success
- choose a command plugin when the feature deserves its own top-level CLI
- choose a TLS signer plugin only for advanced mTLS integration

## The Public Helper Package

Plugin authors should build against the public
`plugin` package in the repository.

The helpers you will reach for most often are:

- `plugin.WriteMessage` and `plugin.ReadMessage` for self-delimiting CBOR messages
- `plugin.WriteManifest` and `plugin.WriteCommands` for startup responses
- `plugin.HandleStartupFlags` for `--rsh-plugin-manifest` and
  `--rsh-plugin-commands`
- `plugin.Run` for simple command plugins
- `plugin.CommandClient` for command plugins that delegate HTTP and terminal
  output back to Restish

## Smallest Formatter Plugin

Formatter plugins are a good first plugin because they only need to read
formatter messages from stdin and write final bytes to stdout.

```go
package main

import (
	"fmt"
	"os"

	"github.com/rest-sh/restish/v2/plugin"
)

func main() {
	manifest := plugin.Manifest{
		Name:              "hello-format",
		Version:           "0.1.0",
		Description:       "Example formatter plugin",
		RestishAPIVersion: 2,
		Hooks:             []string{"formatter"},
		FormatterNames:    []string{"hello"},
	}
	if plugin.HandleStartupFlags(os.Stdout, manifest, nil) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)
	var req plugin.FormatterRequest
	if err := dec.ReadMessage(&req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for {
		switch req.Event {
		case "start":
			if req.Response.Body != nil {
				fmt.Fprintf(os.Stdout, "hello: %#v\n", req.Response.Body)
			}
		case "item":
			fmt.Fprintf(os.Stdout, "hello: %#v\n", req.Response.Body)
		case "end":
			return
		}
		if err := dec.ReadMessage(&req); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
```

Build it as `restish-hello-format`, put it on `PATH`, then run:

```bash
restish https://httpbin.org/json -o hello
```

## Smallest Command Plugin

Command plugins contribute top-level commands and can delegate authenticated
HTTP back to the host instead of building their own client stack.

```go
package main

import "github.com/rest-sh/restish/v2/plugin"

func main() {
	manifest := plugin.Manifest{
		Name:              "hello-cmd",
		Version:           "0.1.0",
		Description:       "Example command plugin",
		RestishAPIVersion: 2,
		Hooks:             []string{"command"},
	}
	commands := []plugin.CommandDecl{
		{Name: "hello", Short: "Print a greeting"},
	}

	plugin.Run(manifest, commands, func(command string, args []string, c *plugin.CommandClient) error {
		return c.Stdout([]byte("hello from plugin\n"))
	})
}
```

That binary will show up as:

```bash
restish hello
```

To make authenticated delegated requests from a command plugin:

```go
resp, err := c.Do(&plugin.HTTPRequestMsg{
	Type:   plugin.MsgTypeHTTPRequest,
	Method: "GET",
	URI:    "myapi/users",
})
```

Restish handles auth, TLS, retries, cache settings, and response normalization
before the reply comes back to your plugin.

## Local Development Loop

1. Build the plugin binary with a `restish-` prefix.
2. Put it on `PATH` or install it with `restish plugin install ./restish-name`.
3. Check startup output directly:

```bash
./restish-name --rsh-plugin-manifest
./restish-name --rsh-plugin-commands
```

4. Use `restish plugin list` to confirm discovery.
5. Use `restish plugin debug <name> ...` when you need to inspect CBOR traffic.

If you are iterating on operator-facing behavior, also test the plugin through
the real user command path instead of only through startup flags.

## Good Reference Implementations

- `cmd/restish-csv/main.go` for a small formatter hook plugin
- `cmd/restish-mcp/main.go` for a real command plugin
- `internal/cli/testdata/cmdplugin/main.go` for a tiny command-plugin test fixture
- [Design Records](/docs/contributing/design-records/) for hook payload shapes and command-plugin protocol details

## Operator Path vs Author Path

Two common confusions:

- `restish plugin install ./restish-name` is for users and local testing
- writing the plugin source code is a separate author workflow

If you are documenting or shipping a plugin for operators, make sure you cover:

- how the binary is installed or discovered
- what top-level commands appear after install
- which config keys or profiles the plugin uses
- one full working example

## Common Pitfalls

- Plugin executables must be named `restish-<name>`.
- Manifests, command declarations, and runtime messages are self-delimiting
  CBOR data items, not length-prefixed frames.
- Command-plugin runtime messages should use `plugin.ReadMessage` and
  `plugin.WriteMessage`.
- Formatter plugins write final bytes to stdout directly; they do not send a
  CBOR reply envelope.
- Formatter plugins receive a sequence of `formatter` messages with
  `start`/`item`/`end` events, not a single one-shot request.
- If you start a subprocess or long-lived goroutine inside a plugin, make sure
  it exits cleanly when stdin closes.
