---
title: Plugin Quickstart
linkTitle: Quickstart
weight: 25
description: Build the smallest useful plugin and verify that Restish discovers it.
---

This author-focused quickstart is for plugin developers. Operators should start
with [Install and Use Plugins](../install-and-use/).

## Choose The Smallest Plugin Type

- Formatter hook: easiest way to add an output format.
- Command plugin: best when the feature needs its own command.
- TLS signer: only for external client-key signing.

## Build And Install

Use an existing first-party plugin as the template for your type:

```bash
go build ./cmd/restish-csv
restish plugin install ./restish-csv
restish plugin list
```

Verify behavior:

```bash
restish api.rest.sh/images -o csv
```

## Debug Protocol Messages

```bash
restish plugin debug ./restish-csv
```

## Authoring Rules

- Keep stdout reserved for protocol messages unless the protocol says otherwise.
- Send human diagnostics to stderr.
- Redact secrets.
- Delegate HTTP to Restish from command plugins.
- Keep operator documentation separate from protocol details.

## Related Pages

- [Hook Plugins](../hook-plugins/)
- [Command Plugins](../command-plugins/)
- [Plugin Manifest](/docs/reference/plugin-manifest/)
- [Plugin Messages](/docs/reference/plugin-messages/)
