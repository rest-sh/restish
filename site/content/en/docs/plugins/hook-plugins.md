---
title: Hook Plugins
linkTitle: Hook Plugins
weight: 30
description: Author auth, request middleware, response middleware, loader, and formatter hook plugins.
---

Hook plugins are short-lived extensions. Restish starts them for one focused
job, reads the result, and continues the host request pipeline.

## Hook Types

- `auth`: produce or mutate request auth.
- `request`: inspect or mutate outgoing requests.
- `response`: inspect or mutate incoming responses.
- `loader`: load API descriptions from additional content types or sources.
- `formatter`: render normalized responses as new output formats.

## Formatter Example

A CSV formatter should receive normalized response data and return terminal or
file output without owning HTTP, auth, retry, cache, or TLS behavior.

```bash
restish plugin list
restish https://api.rest.sh/images -o csv
```

## Middleware Example Shape

```json
{
  "type": "request",
  "request": {
    "method": "GET",
    "uri": "https://api.rest.sh/items"
  }
}
```

## Pitfalls

- Keep hooks narrow and deterministic.
- Do not write human output to stdout unless the hook is a formatter.
- Redact secrets in errors and logs.
- Prefer host-provided request and response models over custom HTTP stacks.

## Related Pages

- [Plugin Manifest](/docs/reference/plugin-manifest/)
- [Plugin Messages](/docs/reference/plugin-messages/)
- [Command Plugins](../command-plugins/)
