---
title: Plugin Manifest
linkTitle: Plugin Manifest
weight: 42
description: Reference for the manifest metadata Restish expects from plugins.
---

Every Restish plugin must expose a manifest. Restish fetches it by invoking the
plugin with `--rsh-plugin-manifest`.

The manifest may be returned as CBOR or JSON.

## Required Fields

- `name`: plugin name
- `restish_api_version`: plugin protocol version

## Optional Fields

- `version`: plugin version string. Strongly recommended for discovery and debugging.
- `description`: short human-readable summary. Strongly recommended for `plugin list`.
- `hooks`: declared plugin hook types
- `formatter_names`: output format names provided by a formatter plugin
- `loader_content_types`: content types handled by a loader plugin

## Example

```json
{
  "name": "hello-format",
  "version": "0.1.0",
  "description": "Example formatter plugin",
  "restish_api_version": 2,
  "hooks": ["formatter"],
  "formatter_names": ["hello"]
}
```

## Hook Names

Current hook names include:

- `auth`
- `request-middleware`
- `response-middleware`
- `loader`
- `formatter`
- `command`
- `tls-signer`

## Compatibility Rules

`restish_api_version` tells Restish which plugin protocol version the plugin
expects.

- missing or invalid values are rejected
- newer versions may still load with a warning
- version mismatches are an important debugging clue when a plugin seems to be
  discovered but not behaving correctly

## Related Pages

- [Install and Use Plugins](/docs/plugins/install-and-use/)
- [Plugin Reference](../plugins/)
- [Plugin Message Reference](../plugin-messages/)
- [Plugin Quickstart](/docs/plugins/quickstart/)
