---
title: Filtering
linkTitle: Filtering
weight: 60
description: Filter and project response data in Restish with shorthand queries and jq.
---

# Filtering

Restish supports response filtering so users can focus on the fields they care
about without piping every response through another tool.

Filtering happens after response normalization and before formatting. That means
your filter runs against a stable document structure rather than raw transport
objects.

## The Available Roots

Restish filters operate on a normalized response document with these roots:

- `proto`
- `status`
- `headers`
- `links`
- `body`
- `@` for the full document

## Two Filter Languages

Restish supports:

- shorthand path syntax for direct field access
- jq for richer transformations and predicates

By default, filter mode is `auto`:

- if the expression starts with `body`, `headers`, `links`, `status`, `proto`,
  or `@`, Restish treats it as shorthand
- otherwise, Restish treats it as jq

## Common Shorthand Filters

```bash
restish get https://api.example.com/items -f body.items[0].name
restish get https://api.example.com/items -f headers.Content-Type
restish get https://api.example.com/items -f links.next
```

Shorthand is best when you want direct access to a known field quickly.

## Common jq Filters

```bash
restish get https://api.example.com/items -f '.body.items[] | select(.active) | .name'
restish get https://api.example.com/items -f '.body.items | length'
```

jq is the better choice when you need selection, transformation, or aggregation.

## Raw Output

For shell-friendly output, combine a filter with `--rsh-raw`:

```bash
restish get https://api.example.com/items -f '.body.items[] | .name' --rsh-raw
```

That prints simple scalar results without JSON quoting and prints arrays of
scalars one item per line.

## Choosing Between Shorthand And jq

Use shorthand when:

- you want `body.name`
- you need a header value
- you want a pagination link
- you already know the exact path

Use jq when:

- you need `select(...)`
- you want `length`
- you want to reshape arrays or objects
- you are doing more than direct field access

## Why Filtering Feels Consistent

Because filtering happens after normalization:

- body filters work the same way across formatters
- protocol metadata stays accessible when you need it
- sub-value filtering is predictable in scripts

Once you filter down to a sub-value, Restish renders that filtered value rather
than trying to preserve the original raw response bytes.

## Learn More

- [Output](../output/)
- [`docs/design/010-filtering-and-projection.md`](/Users/daniel/src/restish2/docs/design/010-filtering-and-projection.md)

Source material:

- [`docs/design/010-filtering-and-projection.md`](/Users/daniel/src/restish2/docs/design/010-filtering-and-projection.md)
