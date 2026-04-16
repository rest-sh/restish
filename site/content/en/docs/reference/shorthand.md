---
title: Shorthand Syntax
linkTitle: Shorthand Syntax
weight: 30
description: Reference for Restish shorthand input syntax, coercion rules, arrays, file loading, and patch-style updates.
---

Restish shorthand is the structured input language used for request bodies and,
in some contexts, patch-style updates on top of stdin.

Use it when you want to build a small JSON-like structure directly on the
command line instead of creating a separate file first.

## Core Examples

Simple object:

```bash
restish post https://api.rest.sh name: Alice active: true
```

Nested object:

```bash
restish post https://api.rest.sh user.name: Alice user.role: admin
```

Array append:

```bash
restish post https://api.rest.sh tags[]: red tags[]: blue
```

Patch existing stdin:

```bash
echo '{"name":"Alice","role":"user"}' | \
  restish post https://api.rest.sh role: admin
```

## Type Coercion

Unquoted values are coerced when they look like one of the built-in literal
types.

Examples:

- `true` -> boolean
- `false` -> boolean
- `null` -> null
- `123` -> number
- `1.5` -> number
- `2024-01-01T12:00:00Z` -> timestamp-like value

Use quotes when you want the literal string instead:

```bash
restish post https://api.rest.sh value: "true" count: "123"
```

The empty string can be written either as a blank value or as `""`:

```bash
restish post https://api.rest.sh blank1: blank2: ""
```

## Object Construction

Use `.` to create nested objects:

```bash
restish post https://api.rest.sh user.address.city: Honolulu
```

Equivalent body:

```json
{
  "user": {
    "address": {
      "city": "Honolulu"
    }
  }
}
```

## Arrays

Append with `[]`:

```bash
restish post https://api.rest.sh tags[]: red tags[]: blue
```

Set a specific index with `[n]`:

```bash
restish post https://api.rest.sh tags[0]: red tags[1]: blue
```

Nested arrays and objects can be mixed freely:

```bash
restish post https://api.rest.sh users[0].name: Alice users[1].name: Bob
```

## File Loading

Use `@file` to load content from a file.

```bash
restish post https://api.rest.sh spec: @openapi.json
```

For structured files, Restish can load the file as structured data. For plain
text or raw data, the content becomes the value.

This is useful when one field is large but the rest of the body still benefits
from shorthand.

## Stdin Interaction

The main stdin rules are:

- no shorthand args and TTY stdin means no request body
- stdin alone is parsed as structured input when possible
- shorthand args alone build the body
- stdin plus shorthand args treats stdin as the base document and applies the
  shorthand values as a patch

That last rule is one of the most useful productivity features in Restish.

## Shell Quoting Advice

Many shells treat `?`, `[]`, and `*` specially. If your shell is doing glob
expansion before Restish sees the input:

- quote the argument
- or run `restish setup <shell>` for `noglob`-style protection

Example:

```bash
restish post https://api.rest.sh 'tags[]: red' 'tags[]: blue'
```

## When To Use Shorthand vs Files

Use shorthand when:

- the body is small
- the shape is easy to read inline
- you are exploring quickly
- you want to patch structured stdin

Use files or stdin when:

- the document is large
- exact formatting matters
- another tool already produces the payload

## Related Pages

- [Input and Shorthand](/docs/guides/input/)
- [Requests](/docs/guides/requests/)
- [First Request](/docs/getting-started/first-request/)
