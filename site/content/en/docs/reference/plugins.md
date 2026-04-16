---
title: Plugins
linkTitle: Plugins
weight: 40
description: Reference for plugin manifests, hooks, and protocol-level behavior in Restish.
---

Use plugins when Restish's built-in request, output, auth, or TLS behavior is
not enough but you still want to stay inside the Restish workflow model.

## Discovery

Restish discovers plugins from:

1. executables on `PATH` whose filename starts with `restish-`
2. the installed plugin directory, usually `~/.config/restish/plugins/`

Each candidate is invoked with `--rsh-plugin-manifest`, and Restish uses the
returned manifest to decide how the plugin participates.

## Plugin Categories

### Hook Plugins

Best for one-shot extension points such as auth, middleware, loaders, and
formatters.

### Command Plugins

Best for longer-lived workflows that deserve their own top-level command, such
as `restish bulk` or `restish mcp`.

### TLS Signer Plugins

Best for mutual TLS setups where the private key must remain outside the
Restish process.

## User-Facing Plugin Commands

```bash
restish plugin list
restish plugin install ./restish-my-plugin
restish plugin remove restish-my-plugin
restish plugin debug restish-my-plugin
```

Those commands let you inspect discovered plugins, copy a local plugin into the
default plugin directory, remove an installed plugin, or debug CBOR traffic.

## Restricting Auto-Discovery

Use `allowed_plugins` in `restish.json` when you want Restish to load only a
known set of plugin executables:

```json
{
  "allowed_plugins": ["restish-bulk", "restish-csv"]
}
```

## Where To Go Next

- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Plugins Section](/docs/plugins/)
- [Plugin Manifest Reference](../plugin-manifest/)
- [Plugin Message Reference](../plugin-messages/)
