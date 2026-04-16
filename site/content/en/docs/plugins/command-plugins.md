---
title: Command Plugins
linkTitle: Command Plugins
weight: 30
description: Add new top-level Restish workflows with long-lived command plugins.
---

Command plugins add top-level commands such as `restish bulk` and keep running
while they exchange messages with the host.

Choose a command plugin when your feature needs:

- a top-level command
- multiple HTTP requests in one workflow
- progress updates while work is running
- optional passthrough stdin/stdout for interactive sessions

If a single request and a single reply are enough, prefer a
[hook plugin](../hook-plugins/).

## Startup Flow

Command plugins declare the `command` hook in their manifest. At CLI startup
Restish invokes the plugin with `--rsh-plugin-commands` and reads command
declarations such as:

```json
{
  "commands": [
    {
      "name": "bulk",
      "short": "Client-side bulk resource management",
      "long": "Manage many API resources as files",
      "passthrough_stdio": false
    }
  ]
}
```

Each declaration becomes a root command on the host CLI.

When the user runs that command, Restish starts the plugin and sends:

```json
{
  "type": "init",
  "command": "bulk",
  "args": ["init", "https://api.example.com/all-items"]
}
```

## Messages From Plugin To Restish

The current command-plugin protocol includes:

- `http-request`: ask Restish to perform an HTTP request
- `api-spec`: ask Restish to resolve a registered API spec
- `list-apis` and `list-profiles`: inspect host configuration
- `config-read`: read effective API/profile/plugin config
- `prompt` and `confirm`: ask the user for input
- `response`: ask Restish to format and print a normalized response
- `stdout-data` and `stderr-data`: write raw terminal output
- `progress`, `spinner`, `log`, and `warn`: print status messages on stderr
- `done`: finish with an exit code

## Messages From Restish To Plugin

Restish replies with:

- `http-response`
- `api-spec-response`
- `list-apis-response`
- `list-profiles-response`
- `config-read-response`
- `prompt-response`
- `confirm-response`
- `stdin-data` and `stdin-close` when passthrough stdio is enabled

## Why Delegated HTTP Matters

Command plugins should usually ask Restish to make HTTP requests instead of
building a second HTTP client stack themselves.

That keeps plugin workflows aligned with the normal host behavior:

- profile and base URL resolution
- authentication
- TLS signer integration
- retries and caching
- normalized response handling

That is why user-facing command plugins such as `restish bulk` and
`restish mcp` can feel like part of the main CLI instead of separate tools with
their own auth and transport stacks.

## Minimal Go Example

```go
package main

import (
	"github.com/danielgtaylor/restish/v2/plugin"
)

func main() {
	plugin.Run(
		plugin.Manifest{
			Name:              "hello-command",
			Version:           "0.1.0",
			Description:       "Example command plugin",
			RestishAPIVersion: 2,
			Hooks:             []string{"command"},
		},
		[]plugin.CommandDecl{{
			Name:  "hello-plugin",
			Short: "Print a hello message",
		}},
		func(command string, args []string, client *plugin.CommandClient) error {
			_ = client.Progress("running hello-plugin")
			if err := client.Stdout([]byte("hello from command plugin\n")); err != nil {
				return err
			}
			return nil
		},
	)
}
```

## Real User Examples

Command plugins are not only for authors. The current repository includes
first-party command plugins that expose actual user workflows:

- `restish bulk ...` for local bulk resource management
- `restish mcp ...` for serving registered APIs over MCP

Those are good reference points for deciding whether a feature should be a
command plugin at all.

## Pitfalls

- The host disables its own flag parsing for contributed commands. The plugin
  is responsible for its own subcommands, flags, and help text.
- `done` is required. If the plugin exits without it, Restish treats that as a
  failed command.
- Use delegated `http-request` whenever you want Restish auth, retries, cache,
  or normalization behavior.
- Use raw `stdout-data` and `stderr-data` only when the plugin truly owns the
  terminal output for that step.

## When Not To Use A Command Plugin

Prefer a hook plugin instead when:

- one request and one reply are enough
- you only need auth, middleware, loader, or formatter behavior
- the feature does not need its own top-level command surface

## Related Pages

- [Hook Plugins](../hook-plugins/)
- [Plugin Manifest](../reference/plugin-manifest/)
- [Plugin Message Reference](../reference/plugin-messages/)
- [Plugin Quickstart](/docs/plugins/quickstart/)
- [Design Record 020](/docs/contributing/design-records/)
