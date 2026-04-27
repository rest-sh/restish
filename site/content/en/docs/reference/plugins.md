---
title: Plugins
linkTitle: Plugins
weight: 60
description: Reference for plugin discovery, installation, operating model, and plugin categories.
---

Plugins extend Restish without replacing the host runtime. Operators should
start with install and verification; authors should read manifest and message
references.

## Operator Commands

```bash
restish plugin list
restish plugin install ./restish-csv
restish plugin remove restish-csv
restish plugin debug ./restish-csv
```

## Plugin Categories

- Hook plugins: auth, request middleware, response middleware, loaders, formatters.
- Command plugins: top-level workflows such as `bulk` and `mcp`.
- TLS signer plugins: external client-certificate signing.

## Discovery Expectations

A plugin must be executable, discoverable, and compatible with the v2 protocol.
When discovery fails, check file permissions, install location, manifest fields,
and protocol version.

## Related Pages

- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Plugin Manifest](../plugin-manifest/)
- [Plugin Messages](../plugin-messages/)
