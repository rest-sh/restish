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
- `request-middleware`: inspect or mutate outgoing requests.
- `response-middleware`: inspect or mutate incoming responses.
- `loader`: load API descriptions from additional content types or sources.
- `formatter`: render normalized responses as new output formats.

## Formatter Example

A CSV formatter should receive normalized response data and return terminal or
file output without owning HTTP, auth, retry, cache, or TLS behavior.

```bash
restish plugin list
restish api.rest.sh/images -o csv
```

## Middleware Example Shape

```json
{
  "type": "request-middleware",
  "request": {
    "method": "GET",
    "uri": "https://api.rest.sh/items"
  }
}
```

Request middleware can update headers by returning strings, arrays of strings,
or `null`; `null` deletes that header from the prepared request. Response
middleware can return `response.headers` to replace normalized response headers,
so include any original headers the plugin wants to keep.

Response middleware can also return a `follow` object with `method`, `uri`,
optional `headers`, optional `body`, and optional `content_type`. Restish sends
that follow-up request through the normal host pipeline, including TLS, retries,
auth for same-origin follows, and credential stripping for cross-host follows.
Response middleware runs when Restish renders, filters, collects, or explicitly
prints an interpreted response. It is skipped for plain raw downloads such as
`restish URL > file`, where Restish preserves the original response body bytes.

Auth hooks run after built-in auth has prepared the request. For generated
operations that require more than one credential, Restish applies all selected
credentials and invokes auth hooks once with the final request. In that
multi-credential case the hook input does not include individual credential
params; single-credential auth continues to include params with secrets redacted
unless the manifest opts into auth secrets.

## Pitfalls

- Keep hooks narrow and deterministic.
- Do not write human output to stdout unless the hook is a formatter.
- Redact secrets in errors and logs.
- Prefer host-provided request and response models over custom HTTP stacks.

## Related Pages

- [Plugin Quickstart](../quickstart/)
- [Plugin Manifest](/docs/reference/plugin-manifest/)
- [Plugin Messages](/docs/reference/plugin-messages/)
- [Command Plugins](../command-plugins/)
