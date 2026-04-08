---
title: Plugins
linkTitle: Plugins
weight: 40
description: Understand the three plugin types in Restish and when to use each one.
---

# Plugins

Restish v2 supports three plugin shapes:

- hook plugins for request-time extensions
- command plugins for long-lived workflows
- TLS signer plugins for external client-key signing

Plugins are a major part of the v2 story, so this docs site gives them a
dedicated top-level section instead of hiding them inside contributor material.

## How Plugins Are Discovered

Restish discovers plugins as executables named `restish-<name>`.

It looks in:

1. executables on `PATH`
2. the configured plugin directory, usually `~/.config/restish/plugins/`

That makes plugins easy to ship as standalone binaries instead of requiring a
custom Restish build.

## Hook Plugins

Hook plugins are the smallest integration point. They are short-lived and best
for focused request-time behavior such as:

- auth
- request middleware
- response middleware
- custom spec loaders
- custom output formatters

Choose a hook plugin when one request in and one reply out is the right model.

## Command Plugins

Command plugins add longer-lived top-level commands such as `restish mcp ...`.

They are a better fit when the plugin needs:

- multiple round trips
- delegated HTTP calls back through Restish
- progress or terminal interactions
- a richer workflow than a single request-time hook

## TLS Signer Plugins

TLS signer plugins are the specialized path for mutual TLS when the private key
must stay outside the Restish process.

This is the plugin type to choose for HSMs, hardware-backed keys, or external
signing systems.

## Why There Are Multiple Plugin Types

The split is intentional. Each plugin type has different lifecycle and trust
requirements:

- hook plugins stay simple
- command plugins support conversations
- TLS signer plugins isolate private-key operations

That separation keeps each protocol clearer than a single all-purpose plugin
model would be.

## User-Facing Plugin Commands

Restish exposes a few plugin management commands directly:

```bash
restish plugin list
restish plugin install ./restish-myplugin
restish plugin remove restish-myplugin
restish plugin debug myplugin
```

Use `plugin debug` when you need to inspect a plugin's CBOR traffic and runtime
behavior.

## Config And Trust Boundaries

Plugins are additive to the main CLI model, not replacements for it. Restish
still owns:

- the main request pipeline
- config loading
- profile behavior
- output selection

Plugins extend specific seams where out-of-process behavior is the better fit.

## Related Guides

- [Plugin Quickstart](../plugins/quickstart/)
- [Hook Plugins](../plugins/hook-plugins/)
- [Command Plugins](../plugins/command-plugins/)
- [TLS Signer Plugins](../plugins/tls-signer-plugins/)

## Source Material

- [`docs/design/018-plugin-architecture-overview.md`](/Users/daniel/src/restish2/docs/design/018-plugin-architecture-overview.md)
