---
title: Shorthand Syntax
linkTitle: Shorthand Syntax
weight: 30
description: Dense reference for all Restish shorthand input syntax — type coercion, arrays, objects, comments, base64, move, unset, and API configuration use cases.
---

Restish shorthand is the structured input language used for request bodies,
`restish api add` / `restish api set` config patches, and filter query paths.

This page is the complete reference. For an orientation with worked examples
see [Input and Shorthand](/docs/guides/input/).

The underlying library is documented at
https://github.com/danielgtaylor/shorthand.

---

## Syntax Overview

Each argument is a `key: value` pair. Multiple pairs produce an object. A bare
value with no key is treated as the entire expression. Keys use `.` for nesting
and `[n]` for array indexing.

```
key: value
parent.child: value
array[]: item
array[0]: first
```

Multiple arguments are joined with a space before parsing, so these are
equivalent:

```bash
restish post https://api.rest.sh name: Alice age: 30
restish post https://api.rest.sh 'name: Alice age: 30'
```

---

## Type Coercion

Unquoted values are coerced to the most specific matching type:

| Input | Type | Notes |
|-------|------|-------|
| `true` / `false` | boolean | case-sensitive |
| `null` | null | |
| `123` | integer | |
| `1.5` | float | |
| `2024-01-01T12:00:00Z` | string | passed through; not a Go time.Time |
| anything else | string | |

Quote with `"..."` to force a string regardless of value:

```bash
restish post https://api.rest.sh enabled: "true" count: "123"
```

---

## Arrays

Append with `[]`:

```bash
tags[]: red tags[]: blue
# → {"tags": ["red", "blue"]}
```

Set by index with `[n]`:

```bash
tags[0]: red tags[1]: blue
# → {"tags": ["red", "blue"]}
```

Mixing append and indexed is allowed; indexed positions set in order.

Nested objects inside arrays:

```bash
users[0].name: Alice users[0].role: admin
# → {"users": [{"name": "Alice", "role": "admin"}]}
```

---

## Unset / Delete A Key — `undefined`

Set a key to the special literal `undefined` to remove it from the output when
patching an existing document.

```bash
echo '{"name":"Alice","role":"user"}' | \
  restish post https://api.rest.sh role: undefined
# body sent: {"name": "Alice"}
```

This is the main way to delete a field during a piped patch or an
`api set` config edit:

```bash
restish api set myapi 'operation_base: undefined'
```

---

## Move A Key — `^`

Prefix a value with `^` to move a value from another key path rather than
providing a literal value.

```bash
echo '{"old":"value"}' | \
  restish post https://api.rest.sh new: ^old
# body sent: {"new": "value"}
```

The source key is removed from the document after the move.

---

## Comments — `//`

Lines starting with `//` are treated as comments and ignored. This is useful
when shorthand appears in a config file or is generated programmatically.

```
// set the display name
name: Alice
// override the role
role: admin
```

Comments are stripped before parsing; they cannot appear inline at the end of
a value line.

---

## Base64 File Load — `%`

Prefix a file path with `%` to load the file and encode its contents as a
base64 string value.

```bash
restish post https://api.rest.sh icon: %./logo.png
# → {"icon": "<base64-encoded PNG bytes>"}
```

The standard `@` prefix loads a file and inlines its content as structured
data (parsed JSON/YAML) or as a raw string. Use `%` when you need the binary
content base64-encoded instead.

---

## File Load — `@`

Prefix a value with `@` to load the file at that path and use its content as
the value.

```bash
restish post https://api.rest.sh config: @config.json
```

For structured files (`.json`, `.yaml`, `.cbor`, etc.), the content is parsed
and inlined as a structured value. For unrecognized types, the content becomes
a string.

---

## The `j` Helper — Inline Raw JSON

Prefix a value with `j` immediately followed by a JSON literal to inline it
verbatim without the usual coercion rules.

```bash
restish post https://api.rest.sh meta: j{"extra":true}
# → {"meta": {"extra": true}}
```

This is useful when you need a literal JSON object or array as a field value
inside a larger shorthand expression, without creating a separate file.

---

## Patch Mode

When shorthand arguments are combined with stdin, stdin provides the base
document and the shorthand arguments are applied as a patch over it.

```bash
cat existing.json | restish post https://api.rest.sh role: admin
```

Only the keys mentioned in the patch are changed. All other keys pass through
unchanged from the base document. Use `undefined` to delete keys.

---

## Using Shorthand In `api add` And `api set`

The `api add` and `api set` commands accept shorthand to configure profile
fields, headers, and other settings without editing the JSON file directly.

Add an API with a bearer token header:

```bash
restish api add myapi https://api.example.com \
  'profiles.default.headers.Authorization: "Bearer abc123"'
```

Set a persistent header on an existing API:

```bash
restish api set myapi 'profiles.default.headers.X-Team: platform'
```

Remove a field:

```bash
restish api set myapi 'operation_base: undefined'
```

The same `undefined` and `^` semantics apply in this context.

---

## Filter Query Paths

Shorthand dot-path syntax is also the default query language for `--rsh-filter`
when the expression starts with one of the recognized roots (`body`, `headers`,
`links`, `status`, `proto`). For example:

```bash
restish get https://api.rest.sh/ --rsh-filter body.images[0].url
```

Expressions that do not start with a recognized root are passed to jq instead.
See [Query Syntax](/docs/reference/query-syntax/) for full details on both
languages.

---

## Related Pages

- [Input and Shorthand](/docs/guides/input/) — orientation and worked examples
- [Query Syntax](/docs/reference/query-syntax/) — shorthand and jq filter paths
- [API Management](/docs/reference/api-management/) — `api add` and `api set`
- [Requests](/docs/guides/requests/)
- [Shorthand library README](https://github.com/danielgtaylor/shorthand)
