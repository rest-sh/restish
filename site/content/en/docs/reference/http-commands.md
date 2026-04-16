---
title: Generic HTTP Commands
linkTitle: HTTP Commands
weight: 11
description: Reference for Restish generic HTTP verbs and the bare-URL GET shortcut.
---

Restish includes these generic HTTP verbs:

- `get`
- `post`
- `put`
- `patch`
- `delete`

These work without API registration.

## Examples

```bash
restish get https://api.rest.sh/
restish post https://api.rest.sh name: Alice
restish patch https://api.rest.sh/types string: changed
restish delete https://api.example.com/items/123
```

## Bare URL Shortcut

A bare URL is treated as `GET`:

```bash
restish https://api.rest.sh/
```

This is equivalent to:

```bash
restish get https://api.rest.sh/
```

## Shared Behavior

Generic HTTP commands still participate in the normal Restish runtime:

- profiles
- auth
- TLS
- filtering
- output formats
- retries and cache

## Related Pages

- [Commands](../commands/)
- [Global Flags](../global-flags/)
- [Requests](/docs/guides/requests/)
