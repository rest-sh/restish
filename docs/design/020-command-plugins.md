# Command Plugins

## Summary

Command plugins are long-lived subprocesses that contribute top-level CLI
commands and can converse with Restish over a multi-message CBOR protocol while
they run.

This plugin type is for workflow-oriented features that do not fit naturally
into a single request hook.

## Problem

Some extensions need to:

- expose their own user-facing subcommands
- coordinate multiple HTTP requests
- stream progress or warnings while work is running
- optionally pass stdin/stdout through as part of an interactive session

Trying to model that as repeated hook invocations would be awkward and would
force the plugin to reinvent parts of the CLI lifecycle itself.

## Design

A command plugin declares the `command` hook in its manifest. Restish then
discovers contributed commands in a separate startup phase by invoking the
plugin with `--rsh-plugin-commands`.

That command returns a CBOR payload describing one or more commands, including:

- `name`
- `short`
- `long`
- `passthrough_stdio`

Each declaration becomes a Cobra command on the root CLI.

When the user runs one of those commands, Restish starts the plugin and sends
an initial message:

```json
{
  "type": "init",
  "command": "<declared name>",
  "args": ["..."]
}
```

From there, the plugin and Restish exchange structured CBOR messages until the
plugin sends `done` or dies.

### Messages From Plugin To Restish

The current implementation handles these message types:

- `http-request` to ask Restish to perform an HTTP call
- `api-spec` to ask Restish to resolve a registered API spec
- `response` to ask Restish to format and print a normalized response
- `stdout-data` and `stderr-data` to write raw bytes directly
- `progress`, `spinner`, and `log` to print status text on stderr
- `warn` to print a warning-prefixed message
- `done` to terminate with an exit code

### Messages From Restish To Plugin

Restish currently sends:

- `init` when the command starts
- `http-response` after handling an `http-request`
- `api-spec-response` after handling an `api-spec`
- `stdin-data` and `stdin-close` when `passthrough_stdio` is enabled

### Delegated HTTP

The most important design choice is that command plugins delegate HTTP back to
Restish instead of constructing their own transport stack. When a plugin sends
`http-request`, Restish performs the request through the normal request path,
including:

- profile and API-base-URL resolution
- TLS signer resolution
- request middleware
- retries, caching, and output normalization

The reply sent back to the plugin is the normalized response shape, not a raw
`*http.Response`.

That keeps workflows consistent with the rest of the CLI and avoids creating a
second HTTP client implementation inside every plugin.

### Output Ownership

Command plugins have two output modes:

- ask Restish to print a normalized `response` using the current formatter and
  filter rules
- emit raw `stdout-data` or `stderr-data` bytes directly

This lets a workflow choose between "behave like a normal Restish command" and
"own the terminal output myself" on a message-by-message basis.

### Terminal Context

Restish also passes simple terminal context flags on plugin startup:

- `--rsh-stdout-tty=<bool>`
- `--rsh-stderr-tty=<bool>`
- `--rsh-color=<bool>`

That gives plugins enough signal to adapt interactive behavior without coupling
them tightly to the terminal implementation.

## Alternatives Considered

### Run command plugins as ordinary child processes with no protocol

That would be simpler mechanically, but the plugin would lose access to
Restish-managed HTTP, config, and formatting. The protocol is what keeps the
workflow integrated.

### Let command plugins make HTTP requests directly

This would duplicate auth, retry, caching, TLS, and normalization logic across
plugins. Delegation back into Restish is a better fit for consistency and
maintainability.

### Model command plugins as many independent hook calls

That would make progress reporting, streamed stdin, and multi-step workflows
far more awkward than keeping one long-lived session.

## Notes

The current implementation is centered in
[`internal/cli/command_plugin.go`](/Users/daniel/src/restish2/internal/cli/command_plugin.go),
with end-to-end examples in
[`internal/cli/testdata/cmdplugin/main.go`](/Users/daniel/src/restish2/internal/cli/testdata/cmdplugin/main.go)
and tests in
[`internal/cli/command_plugin_test.go`](/Users/daniel/src/restish2/internal/cli/command_plugin_test.go).

One detail worth preserving is that command plugins add commands at root-command
construction time, not through late dynamic dispatch. That keeps help output,
completion, and command discovery aligned with built-in commands.
