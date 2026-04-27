---
title: Shorthand
linkTitle: Shorthand
weight: 34
description: Reference for Restish shorthand request bodies, stdin patches, file loading, arrays, and scalar values.
---

Shorthand builds structured request bodies from CLI arguments. It is designed
for common JSON-shaped API input without requiring temporary files.

## Objects

```bash
restish post https://api.rest.sh/post 'name: Alice, enabled: true' count: 3
restish post https://api.rest.sh/post 'user.name: Alice, user.active: true'
```

## Arrays

```bash
restish post https://api.rest.sh/post 'tags[]: docs' 'tags[]: cli'
restish post https://api.rest.sh/post 'items[0].name: first' 'items[1].name: second'
```

Quote brackets in shells that expand them.

## Scalars

Common values are coerced where appropriate:

```bash
restish post https://api.rest.sh/post 'enabled: true, count: 3, price: 12.5'
```

Force strings with quotes when exact text matters:

```bash
restish post https://api.rest.sh/post 'enabled: "true", missing: "null", blank: ""'
```

## File Loading

```bash
restish post https://api.rest.sh/post payload: @payload.json
restish post https://api.rest.sh/post note: @message.txt
restish post https://api.rest.sh/post encoded: %SGVsbG8=
```

Structured files are parsed when possible. Use quoting or explicit string
handling for literal `@` values.

## Patching Stdin

```bash
echo '{"name":"Alice","role":"user"}' | restish post https://api.rest.sh/post role: admin
```

Stdin becomes the base document and shorthand arguments patch it.

## Related Pages

- [Input and Shorthand](/docs/guides/input/)
- [Query Syntax](../query-syntax/)
- [Content Types](../content-types/)
