---
title: Plugin Quickstart
linkTitle: Quickstart
weight: 10
description: Build your first Restish plugin and choose the right plugin type.
---

# Plugin Quickstart

This guide is the shortest path from "I want to extend Restish" to a working
plugin binary.

## Choose a Plugin Type

- Hook plugin: one request in, one reply out. Best for auth, request
  middleware, response middleware, custom spec loaders, and output formatters.
- Command plugin: a long-lived top-level command such as `restish mcp ...` that
  can delegate HTTP calls back to the host.
- TLS signer plugin: advanced mutual TLS use cases where the private key must
  stay outside the Restish process.

Start with a hook plugin unless you already know you need a custom command
lifecycle.

## The Public Helper Package

Plugin authors should build against the public
[`plugin`](/Users/daniel/src/restish2/plugin/plugin.go) package.

The helpers you will reach for most often are:

- `plugin.WriteMessage` and `plugin.ReadMessage` for framed CBOR messages
- `plugin.WriteManifest` and `plugin.WriteCommands` for startup responses
- `plugin.HandleStartupFlags` for `--rsh-plugin-manifest` and
  `--rsh-plugin-commands`
- `plugin.Run` for simple command plugins
- `plugin.CommandClient` for command plugins that delegate HTTP and terminal
  output back to Restish

## Smallest Formatter Plugin

Formatter plugins are a good first plugin because they are one-shot and do not
need a reply envelope on stdout.

```go
package main

import (
	"fmt"
	"os"

	"github.com/danielgtaylor/restish/v2/plugin"
)

type formatterRequest struct {
	Type     string `cbor:"type"`
	Format   string `cbor:"format"`
	Response struct {
		Body any `cbor:"body"`
	} `cbor:"response"`
}

func main() {
	manifest := plugin.Manifest{
		Name:              "hello-format",
		Version:           "0.1.0",
		Description:       "Example formatter plugin",
		RestishAPIVersion: 1,
		Hooks:             []string{"formatter"},
		FormatterNames:    []string{"hello"},
	}
	if plugin.HandleStartupFlags(os.Stdout, manifest, nil) {
		return
	}

	var req formatterRequest
	if err := plugin.ReadMessage(os.Stdin, &req); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "hello: %#v\n", req.Response.Body)
}
```

Build it as `restish-hello-format`, put it on `PATH`, then run:

```bash
restish get https://httpbin.org/json -o hello
```

## Smallest Command Plugin

Command plugins contribute top-level commands and can delegate authenticated
HTTP back to the host instead of building their own client stack.

```go
package main

import "github.com/danielgtaylor/restish/v2/plugin"

func main() {
	manifest := plugin.Manifest{
		Name:              "hello-cmd",
		Version:           "0.1.0",
		Description:       "Example command plugin",
		RestishAPIVersion: 1,
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

## Good Reference Implementations

- [`cmd/restish-csv/main.go`](/Users/daniel/src/restish2/cmd/restish-csv/main.go)
  for a small formatter hook plugin
- [`cmd/restish-mcp/main.go`](/Users/daniel/src/restish2/cmd/restish-mcp/main.go)
  for a real command plugin
- [`internal/cli/testdata/cmdplugin/main.go`](/Users/daniel/src/restish2/internal/cli/testdata/cmdplugin/main.go)
  for a tiny command-plugin test fixture
- [`docs/design/019-hook-plugins.md`](/Users/daniel/src/restish2/docs/design/019-hook-plugins.md)
  for hook payload shapes
- [`docs/design/020-command-plugins.md`](/Users/daniel/src/restish2/docs/design/020-command-plugins.md)
  for command-plugin protocol details

## Common Pitfalls

- Plugin executables must be named `restish-<name>`.
- Manifests and command declarations are unframed CBOR, not length-prefixed
  CBOR messages.
- Command-plugin runtime messages are framed CBOR; use `plugin.ReadMessage` and
  `plugin.WriteMessage`.
- Formatter plugins write final bytes to stdout directly after reading the
  request; they do not send a CBOR reply envelope.
- If you start a subprocess or long-lived goroutine inside a plugin, make sure
  it exits cleanly when stdin closes.
