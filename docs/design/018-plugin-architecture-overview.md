# Plugin Architecture Overview

## Summary

Restish v2 supports out-of-process plugins discovered as executables named
`restish-<name>`. Plugins extend the CLI without linking into the binary, while
still reusing Restish's request pipeline, output model, and config layer where
that makes sense.

The plugin system is intentionally split into a few distinct categories rather
than one generic protocol:

- hook plugins for short-lived request/response extension points
- command plugins for long-lived workflow commands
- TLS signer plugins for mTLS private keys that must stay outside the process

## Problem

Restish needs extensibility in a few different directions:

- small request-time hooks such as auth injection or response rewriting
- new spec loaders and output formats
- larger multi-step workflows that deserve their own commands
- hardware-backed or externally managed TLS signing

Those use cases have very different lifecycle and trust requirements. A single
"plugin interface" would either become too vague to reason about or too heavy
for simple integrations.

## Design

The core model is executable discovery plus a manifest.

Plugins are discovered from:

1. executables on `PATH` whose filename starts with `restish-`
2. the configured plugin directory, usually `~/.config/restish/plugins/`

Each candidate is invoked with `--rsh-plugin-manifest`. Restish accepts either
CBOR or JSON for the manifest, then records:

- identity fields such as `name`, `version`, and `description`
- `restish_api_version` for protocol compatibility
- declared `hooks`
- optional `formatter_names`
- optional `loader_content_types`

At CLI startup, Restish discovers plugins once and uses the manifest to decide
how each plugin participates:

- hook declarations make the plugin eligible for request/response hook calls
- `formatter_names` register output formatters available via `-o <name>`
- `loader_content_types` register spec loaders tried before built-in loaders
- the `command` hook enables command discovery through a separate command
  protocol

The transport between Restish and plugins uses plain CBOR messages over
stdin/stdout. Each message is a single self-delimiting CBOR data item — no
length prefix or other framing is added. This means any language with a CBOR
library can implement a plugin without custom framing code. The public
[`plugin`](/Users/daniel/src/restish2/plugin/plugin.go) package provides
`WriteMessage` and `ReadMessage` helpers for Go plugin authors.

One important boundary is that plugins are additive to, not replacements for,
the in-process registry model described earlier in the design docs. Restish
still has built-in registries for auth handlers, loaders, and formatters. The
plugin layer exists for cases where shipping behavior out of process is the
better tradeoff.

## Why Separate Plugin Types

The split is intentional:

- hook plugins optimize for simple, isolated extensions with a small bounded
  message exchange
- command plugins optimize for conversational workflows that may need multiple
  HTTP round-trips and progress reporting
- TLS signer plugins optimize for keeping private-key operations outside the
  Restish process entirely

That separation keeps each protocol smaller and lets the docs talk about clear
behavioral contracts instead of one catch-all message schema.

## Alternatives Considered

### One generic bidirectional plugin protocol for everything

This would reduce the number of plugin categories, but it would also make small
extensions harder to write and reason about. Most auth, middleware, and loader
cases fit a simple one-shot call much better, while formatter hooks only need a
very small session protocol.

### In-process dynamic plugins

That would avoid subprocess overhead, but it would complicate portability,
distribution, and isolation. Executable plugins are much easier to ship and
debug across platforms.

### Only support library extensibility

The registry-based library API is still useful, but it requires building a
custom binary. The plugin layer gives contributors a way to extend Restish
without forking the main executable.

## Notes

The current implementation lives primarily in:

- `internal/plugin/plugin.go` for discovery, manifests, and default directories
- `plugin/plugin.go` for public CBOR message helpers
- `internal/cli/cli.go` for startup registration of plugin loaders and
  formatters
- `internal/cli/plugin_cmd.go` for user-facing `restish plugin ...` commands

The records that follow document each plugin category separately so contributors
can reason about lifecycle, message shape, and tradeoffs one layer at a time.
