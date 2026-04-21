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

Restish discovers executables named `restish-<name>` in this order:

1. executables on `PATH`
2. the installed plugin directory, usually `~/.config/restish/plugins/`

PATH is checked first. A plugin found in both places will load from `PATH`
because the PATH entry shadows the installed copy.

Each discovered executable is loaded once. If two entries on `PATH` share the
same `restish-<name>`, the first one found takes precedence.

## Restricting Discovery with `allowed_plugins`

By default, Restish loads every `restish-<name>` executable it finds in either
location. Use `allowed_plugins` in `restish.json` to restrict loading to an
explicit allowlist:

```jsonc
{
  "allowed_plugins": ["restish-bulk", "restish-csv"]
}
```

When `allowed_plugins` is set:

- only executables whose base name appears in the list are loaded
- PATH-discovered plugins are also subject to the allowlist; a plugin on PATH
  is not automatically trusted just because it was found before the plugin
  directory
- plugins not in the list are silently skipped at startup

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
