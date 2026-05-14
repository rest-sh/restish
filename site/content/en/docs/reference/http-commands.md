---
title: Generic HTTP Commands
linkTitle: HTTP Commands
weight: 11
description: Reference for Restish generic HTTP verbs and bare-URL method inference.
---

Generic HTTP commands work without API registration. They still use Restish
profiles, auth, TLS, retries, cache, filtering, pagination, and output.

## Usage

```bash
restish [flags] <url> [shorthand ...]
restish get [flags] <url>
restish post [flags] <url> [shorthand ...]
restish put [flags] <url> [shorthand ...]
restish patch [flags] <url> [shorthand ...]
restish delete [flags] <url>
restish head [flags] <url>
restish options [flags] <url>
```

## Common Examples

```bash
restish api.rest.sh/get
restish api.rest.sh/post 'name: Alice, enabled: true'
restish post api.rest.sh/post 'name: Alice, enabled: true'
restish put api.rest.sh/put name: Alice
restish patch api.rest.sh/patch enabled: false
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
restish post api.rest.sh/items 'id: docs-demo, name: Demo, enabled: true, updated: 2026-04-27T00:00:00Z'
restish patch api.rest.sh/items/docs-demo enabled: false
restish delete api.rest.sh/items/docs-demo --rsh-ignore-status-code
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
