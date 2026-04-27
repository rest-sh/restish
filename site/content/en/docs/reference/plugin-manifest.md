---
title: Plugin Manifest
linkTitle: Plugin Manifest
weight: 61
description: Reference for Restish plugin manifest fields.
---

A plugin manifest tells Restish what a plugin is, what protocol version it uses,
and which hooks or command surfaces it provides.

## Core Fields

```json
{
  "name": "restish-csv",
  "version": "0.1.0",
  "description": "Render array responses as CSV",
  "restish_api_version": 2,
  "hooks": ["formatter"]
}
```

| Field | Meaning |
| --- | --- |
| `name` | Stable plugin name. |
| `version` | Plugin version. |
| `description` | Human-facing summary. |
| `restish_api_version` | Host/plugin protocol version. |
| `hooks` | Hook families such as `auth`, `request`, `response`, `loader`, `formatter`, `command`, or `tls-signer`. |

## Guidance

- Keep names stable; config may refer to them.
- Use the narrowest hook set that solves the job.
- Rebuild v1 plugins for the v2 protocol.
- Keep operator docs separate from manifest internals.

## Related Pages

- [Plugin Messages](../plugin-messages/)
- [Hook Plugins](/docs/plugins/hook-plugins/)
- [Command Plugins](/docs/plugins/command-plugins/)
