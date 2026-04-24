---
title: Shorthand Syntax
linkTitle: Shorthand Syntax
weight: 30
description: Dense reference for the shorthand data syntax Restish uses for structured input, patch-style updates, and shorthand query paths.
---

Restish uses the upstream
[shorthand](https://github.com/danielgtaylor/shorthand) language for structured
input. This page keeps the original grammar and examples close to upstream, then
calls out how that syntax shows up in Restish workflows.

For an orientation page with a gentler on-ramp, see
[Input and Shorthand](/docs/guides/input/).

## Syntax Diagram

The upstream shorthand project ships this syntax diagram:

![Shorthand syntax diagram](/images/shorthand-syntax.svg)

Two important reading notes:

- strings can be quoted or unquoted
- the `query` production shown in the diagram is described on
  [Query Syntax](../query-syntax/)

## Keys And Values

At the simplest level, shorthand builds data from key/value pairs separated by
commas:

```bash
restish post https://api.rest.sh 'hello: world, question: how are you?'
```

That produces a structured object equivalent to:

```json
{
  "hello": "world",
  "question": "how are you?"
}
```

This comma rule is easy to miss. If you write one shorthand expression with
multiple pairs, separate them with commas so the parser sees a new pair instead
of continuing the previous string value.

## Types

Shorthand supports JSON types plus a few extra coercions that are useful in CLI
work:

| Type | Example | Notes |
| --- | --- | --- |
| `null` | `null` | JSON `null` |
| `boolean` | `true` | `true` or `false` |
| `number` | `1.5` | integers, floats, scientific notation |
| `string` | `hello` or `"hello"` | quoted or unquoted |
| `bytes` | `%wg==` | unquoted base64 bytes |
| `time` | `2022-01-01T12:00:00Z` | ISO8601 datetime |
| `array` | `[1, 2, 3]` | JSON-like arrays |
| `object` | `{hello: world}` | JSON-like objects |

## Type Coercion

Well-known values are coerced automatically. Quote a value to force it to stay
a string.

With coercion:

```bash
restish post https://api.rest.sh \
  'empty: null, bool: true, num: 1.5, string: hello'
```

As strings:

```bash
restish post https://api.rest.sh \
  'empty: "null", bool: "true", num: "1.5", string: "hello"'
```

Empty strings are valid too:

```bash
restish post https://api.rest.sh 'blank1: , blank2: ""'
```

## Objects

Nested objects use `.` between path segments:

```bash
restish post https://api.rest.sh 'foo.bar.baz: 1'
```

Equivalent JSON:

```json
{
  "foo": {
    "bar": {
      "baz": 1
    }
  }
}
```

You can also group nested object members with `{...}`. In that form the `:` is
optional, so `foo.bar: {...}` and `foo.bar{...}` mean the same thing.

```bash
restish post https://api.rest.sh 'foo.bar{id: 1, count.clicks: 5}'
```

## Arrays

Arrays use `[]` just like JSON:

```bash
restish post https://api.rest.sh '[1, 2, 3]'
```

Array indexes can also be used while building nested values:

```bash
restish post https://api.rest.sh '[0][2][0]: 1'
```

Use an empty index `[]` to append:

```bash
restish post https://api.rest.sh 'a[]: 1, a[]: 2, a[]: 3'
```

## Loading From Files

The `@` preprocessor loads a value from a file.

Text file:

```bash
restish post https://api.rest.sh 'message: @hello.txt'
```

Structured file:

```bash
restish post https://api.rest.sh 'config: @config.json'
```

If the loaded file is structured data such as JSON or YAML, shorthand inlines
it as structured data. Otherwise it becomes text or bytes depending on the
content.

Quote the value when you want a literal leading `@` instead of file loading:

```bash
restish post https://api.rest.sh 'twitter: "@user"'
```

## Bytes

The `%` prefix means base64-encoded bytes, not "read a file and base64 it".

```bash
restish post https://api.rest.sh 'payload: %SGVsbG8='
```

That is useful when the value itself is already base64 data. When you want to
load from a file, use `@...` instead.

## Patch And Partial Update Semantics

Shorthand also supports patch-style updates on existing structured data. In
Restish this matters when stdin provides a base document and the shorthand
expression modifies it.

Patch operations support:

- append with `[]`
- insert before an index with `[^index]`
- remove a field or array item with `undefined`
- move or swap values with `^`

Example setup:

```bash
printf '%s\n' '{"id":1,"tags":["a","b","c"]}' > data.json
```

Append:

```bash
cat data.json | restish post https://api.rest.sh 'tags[]: d'
```

Insert before index `0`:

```bash
cat data.json | restish post https://api.rest.sh 'tags[^0]: z'
```

Remove a field and an array item:

```bash
cat data.json | restish post https://api.rest.sh \
  'id: undefined, tags[1]: undefined'
```

Rename or move data with `^`:

```bash
cat data.json | restish post https://api.rest.sh 'id ^ name'
cat data.json | restish post https://api.rest.sh 'tags[0] ^ tags[-1]'
```

The right-hand side of `^` is a query path to the destination or swap target.

## Comments And Trailing Commas

Shorthand accepts comments and trailing commas in richer multi-line forms:

```text
{
  // This is a comment
  foo.bar[]{
    baz: 1,
    hello: world,
  },
}
```

That is especially useful in config-like shorthand snippets or generated input.

## Restish-Specific Usage

The most common places Restish uses shorthand are:

- request bodies on `post`, `put`, `patch`, and generated API operations
- patching stdin-provided structured input
- shorthand-style filter paths such as `body.items[0].name`

Representative Restish examples:

```bash
restish post https://api.rest.sh 'name: Alice, age: 30'
restish post https://api.rest.sh 'tags[]: red, tags[]: blue'
cat payload.json | restish post https://api.rest.sh 'role: admin'
restish https://api.rest.sh/example -f body.basics.profiles
```

## What This Page Does Not Cover

This page focuses on the shorthand data syntax itself.

Related but separate topics:

- filter queries and selectors:
  [Query Syntax](../query-syntax/)
- Restish stdin/body resolution rules:
  [Input and Shorthand](/docs/guides/input/)
- API config editing workflows:
  [API Management](../api-management/)

## Related Pages

- [Input and Shorthand](/docs/guides/input/)
- [Query Syntax](../query-syntax/)
- [Requests](/docs/guides/requests/)
- [API Management](../api-management/)
- [Upstream shorthand README](https://github.com/danielgtaylor/shorthand/blob/main/README.md)
