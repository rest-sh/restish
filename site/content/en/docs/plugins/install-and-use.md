---
title: Install and Use Plugins
linkTitle: Install and Use
weight: 12
description: Install existing Restish plugins, confirm discovery, and learn the operator workflow separate from plugin authoring.
---

This page is for plugin operators, not plugin authors.

Use it when you already have a built plugin binary and want it to show up in
Restish.

## How Discovery Works

Restish discovers executable files named `restish-<name>` only from the
installed plugin directory, usually `~/.config/restish/plugins/`. Binaries on
`PATH` are ignored for plugin discovery; install plugins explicitly so the
operator-controlled plugin directory is the trust boundary.

## Install a Local Plugin Binary

```bash
restish plugin install ./restish-my-plugin
```

Then confirm discovery:

```bash
restish plugin list
```

## Remove a Plugin

```bash
restish plugin remove restish-my-plugin
```

## Debug a Plugin

When a plugin is discovered but not behaving correctly:

```bash
restish plugin debug restish-my-plugin
```

That prints decoded plugin traffic and is the fastest way to inspect manifest
or protocol issues.

## What Appears After Install

The user-facing result depends on the plugin type:

- formatter hooks add new names to `-o <format>`
- command plugins add new top-level commands such as `restish bulk`
- TLS signer plugins become available to `--rsh-tls-signer`

## Examples

- `restish bulk ...`
- `restish mcp ...`
- `restish https://api.rest.sh/images -o csv`


## Related Pages

- [Plugins Reference](/docs/reference/plugins/)
- [Plugin Quickstart](../quickstart/)
- [Bulk Management](/docs/guides/bulk-management/)
- [MCP](/docs/guides/mcp/)
