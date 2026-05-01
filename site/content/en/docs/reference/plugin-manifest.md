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
  "name": "csv",
  "version": "0.1.0",
  "description": "Render array responses as CSV",
  "restish_api_version": 2,
  "hooks": ["formatter"],
  "formatter_names": ["csv"]
}
```

| Field | Meaning |
| --- | --- |
| `name` | Stable plugin name, without the `restish-` executable prefix. |
| `version` | Plugin version. |
| `description` | Human-facing summary. |
| `restish_api_version` | Minimum host/plugin protocol version required by this plugin. |
| `hooks` | Hook families such as `auth`, `request-middleware`, `response-middleware`, `loader`, `formatter`, `command`, or `tls-signer`. |
| `required_features` | Additive protocol features the host must support before the plugin can run. Unknown optional fields are ignored; unknown required features fail loading. |
| `formatter_names` | Required when `hooks` includes `formatter`; lists output format names. |
| `loader_content_types` | Required when `hooks` includes `loader`; lists source MIME types. |

## Guidance

- Keep names stable; config may refer to them.
- Use the narrowest hook set that solves the job.
- Use `restish_api_version` for the minimum compatible protocol, not the version
  you built with.
- Add `required_features` only when the plugin cannot operate without that
  host behavior.
- Rebuild v1 plugins for the v2 protocol.
- Keep operator docs separate from manifest internals.

## Related Pages

- [Plugin Messages](../plugin-messages/)
- [Hook Plugins](/docs/plugins/hook-plugins/)
- [Command Plugins](/docs/plugins/command-plugins/)
