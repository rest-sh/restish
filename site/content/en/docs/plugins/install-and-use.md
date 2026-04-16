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

Restish discovers executables named `restish-<name>` from:

1. your `PATH`
2. the installed plugin directory, usually `~/.config/restish/plugins/`

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

## Restrict Discovery

If you only want a known allowlist of plugins to load, use
`allowed_plugins` in `restish.json`.

```jsonc
{
  "allowed_plugins": ["restish-bulk", "restish-csv"]
}
```

## Related Pages

- [Plugins Reference](/docs/reference/plugins/)
- [Plugin Quickstart](../quickstart/)
- [Bulk Management](/docs/guides/bulk-management/)
- [MCP](/docs/guides/mcp/)
