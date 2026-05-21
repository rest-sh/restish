---
title: Generic HTTP Commands
linkTitle: HTTP Commands
weight: 11
description: Reference for Restish generic HTTP verbs and bare-URL method inference.
---

Generic HTTP commands work without API registration. They still use Restish
profiles, auth, TLS, retries, cache, filtering, pagination, and output.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen http-commands -->
Generated from the current Cobra command tree.

### `restish get`

Perform an HTTP GET request

Perform an HTTP `GET` request against a full URL or registered API short-name URL.

Use generic HTTP commands for one-off requests, scripting, and APIs that are not registered with `api connect`. Restish still applies global request flags, profile settings for registered API short names, response normalization, output formatting, filtering, retries, caching, pagination, and plugin hooks.

Pass request headers with `-H`, query parameters with `-q`, filters with `-f`, and output format with `-o`.

Usage:

```text
restish get <url>
```

Aliases: `GET`

Examples:

```bash
  restish get https://api.example.com/items
  restish get https://api.example.com/items -f body.items -o table
```


### `restish head`

Perform an HTTP HEAD request

Perform an HTTP `HEAD` request against a full URL or registered API short-name URL.

Use generic HTTP commands for one-off requests, scripting, and APIs that are not registered with `api connect`. Restish still applies global request flags, profile settings for registered API short names, response normalization, output formatting, filtering, retries, caching, pagination, and plugin hooks.

Pass request headers with `-H`, query parameters with `-q`, filters with `-f`, and output format with `-o`.

Usage:

```text
restish head <url>
```

Aliases: `HEAD`

Examples:

```bash
  restish head https://api.example.com/items
```


### `restish options`

Perform an HTTP OPTIONS request

Perform an HTTP `OPTIONS` request against a full URL or registered API short-name URL.

Use generic HTTP commands for one-off requests, scripting, and APIs that are not registered with `api connect`. Restish still applies global request flags, profile settings for registered API short names, response normalization, output formatting, filtering, retries, caching, pagination, and plugin hooks.

Pass request headers with `-H`, query parameters with `-q`, filters with `-f`, and output format with `-o`.

Usage:

```text
restish options <url>
```

Aliases: `OPTIONS`

Examples:

```bash
  restish options https://api.example.com/items
```


### `restish post`

Perform an HTTP POST request

Perform an HTTP `POST` request with optional shorthand, file, or stdin body input.

Use generic HTTP commands for one-off writes, scripting, and APIs that are not registered with `api connect`. Body arguments use Restish shorthand by default; pass `@file.json`, pipe stdin, or set `--rsh-content-type` when you need a specific wire format.

Restish still applies global request flags, response normalization, output formatting, filtering, retries, caching, pagination, and plugin hooks. Unsafe methods are not retried unless you opt in with `--rsh-retry-unsafe`.

Usage:

```text
restish post <url> [body...]
```

Aliases: `POST`

Examples:

```bash
  restish post https://api.example.com/items 'name: Ada, active: true'
  restish post -c json https://api.example.com/items @item.json
```


### `restish put`

Perform an HTTP PUT request

Perform an HTTP `PUT` request with optional shorthand, file, or stdin body input.

Use generic HTTP commands for one-off writes, scripting, and APIs that are not registered with `api connect`. Body arguments use Restish shorthand by default; pass `@file.json`, pipe stdin, or set `--rsh-content-type` when you need a specific wire format.

Restish still applies global request flags, response normalization, output formatting, filtering, retries, caching, pagination, and plugin hooks. Unsafe methods are not retried unless you opt in with `--rsh-retry-unsafe`.

Usage:

```text
restish put <url> [body...]
```

Aliases: `PUT`

Examples:

```bash
  restish put https://api.example.com/items/123 'name: Ada'
  restish put -c json https://api.example.com/items/123 @item.json
```


### `restish patch`

Perform an HTTP PATCH request

Perform an HTTP `PATCH` request with optional shorthand, file, or stdin body input.

Use generic HTTP commands for one-off writes, scripting, and APIs that are not registered with `api connect`. Body arguments use Restish shorthand by default; pass `@file.json`, pipe stdin, or set `--rsh-content-type` when you need a specific wire format.

Restish still applies global request flags, response normalization, output formatting, filtering, retries, caching, pagination, and plugin hooks. Unsafe methods are not retried unless you opt in with `--rsh-retry-unsafe`.

Usage:

```text
restish patch <url> [body...]
```

Aliases: `PATCH`

Examples:

```bash
  restish patch https://api.example.com/items/123 'active: false'
  restish patch https://api.example.com/items/123 -H 'If-Match: abc123' 'active: false'
```


### `restish delete`

Perform an HTTP DELETE request

Perform an HTTP `DELETE` request against a full URL or registered API short-name URL.

Use this for direct delete requests when a generated OpenAPI command is not available or would add friction. Restish still applies global request flags, profile settings for registered API short names, response normalization, output formatting, filtering, and plugin hooks.

By default, HTTP error statuses produce non-zero exit codes. Use `--rsh-ignore-status-code` only when a script intentionally handles those responses.

Usage:

```text
restish delete <url>
```

Aliases: `DELETE`

Examples:

```bash
  restish delete https://api.example.com/items/123
  restish delete https://api.example.com/items/123 --rsh-ignore-status-code
```
<!-- END GENERATED -->

## Common Examples

```bash
restish api.rest.sh/get
restish api.rest.sh/post 'name: Alice, enabled: true'
restish post api.rest.sh/post 'name: Alice, enabled: true'
restish put api.rest.sh/put 'name: Alice'
restish patch api.rest.sh/patch 'enabled: false'
restish delete api.rest.sh/delete --rsh-ignore-status-code
restish head api.rest.sh/head
restish options api.rest.sh/options
```

## Inferred Method

When you omit the verb, Restish chooses the method from the request body:

- no shorthand arguments and no stdin body sends `GET`
- shorthand arguments or stdin body input sends `POST`

Use an explicit verb when the method itself matters, such as `get` for the rare
API that expects a `GET` body or `patch` for partial updates.

For CRUD examples with a path resource, use `/items/{item-id}`:

```bash
ITEM_ID="docs-$(date +%s)"
restish post api.rest.sh/items "id: $ITEM_ID, name: Demo, enabled: true, updated: 2026-04-27T00:00:00Z"
restish patch "api.rest.sh/items/$ITEM_ID" 'enabled: false'
restish delete "api.rest.sh/items/$ITEM_ID" --rsh-ignore-status-code
```

## Output And Errors

Non-2xx HTTP statuses produce status-family exit codes unless
`--rsh-ignore-status-code` is set: `3` for final `3xx`, `4` for final `4xx`,
and `5` for final `5xx`. Response output goes to stdout; verbose
request/response diagnostics go to stderr.

## Related Pages

- [Requests](/docs/guides/requests/)
- [Command Behavior](/docs/guides/command-behavior/)
- [Global Flags](../global-flags/)
