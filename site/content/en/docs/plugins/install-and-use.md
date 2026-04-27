---
title: Install and Use Plugins
linkTitle: Install and Use
weight: 10
description: Install, list, remove, configure, verify, and debug Restish plugins.
---

This is the operator path for using plugins that already exist.

## List Plugins

```bash
restish plugin list
```

Run this first when an output format, command, auth method, or TLS signer is
missing.

## Install A Plugin

```bash
restish plugin install ./restish-csv
```

The plugin must be executable and compatible with the v2 plugin protocol.

## Remove A Plugin

```bash
restish plugin remove restish-csv
```

## Verify Behavior

Formatter example:

```bash
restish https://api.rest.sh/images -o csv
```

Command plugin example:

```bash
restish bulk --help
restish mcp --help
```

## Debug A Plugin

```bash
restish plugin debug ./restish-csv
```

Debug mode prints decoded protocol messages to stderr. Use it when a plugin is
discovered but does not behave as expected.

## Related Pages

- [Plugin Command](/docs/reference/plugin-command/)
- [Example Plugins](../example-plugins/)
- [Troubleshooting](/docs/guides/troubleshooting/)
