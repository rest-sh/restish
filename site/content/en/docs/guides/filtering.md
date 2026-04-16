---
title: Filtering
linkTitle: Filtering
weight: 60
description: Filter and project response data in Restish with shorthand queries and jq.
---

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
restish https://api.rest.sh/example -f body.basics.profiles
restish https://api.rest.sh/ -f headers.Content-Type
restish https://api.rest.sh/images -f links.next
```

Shorthand is best when you want direct access to a known field quickly.

More examples:

```bash
restish https://api.rest.sh/images -f body[0].name
restish https://api.rest.sh/example -f body.volunteer[0].organization
```

Example output:

```json
[
  {
    "network": "Github",
    "url": "https://github.com/danielgtaylor"
  },
  {
    "network": "Dev Blog",
    "url": "https://dev.to/danielgtaylor"
  },
  {
    "network": "LinkedIn",
    "url": "https://www.linkedin.com/in/danielgtaylor"
  }
]
```

```text
application/cbor
```

```text
https://api.rest.sh/images?cursor=abc123
```

## Common jq Filters

```bash
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name'
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

Example output:

```text
Dragonfly macro
```

```text
5
```

jq is the better choice when you need selection, transformation, or aggregation.

More examples:

```bash
restish https://api.rest.sh/images -f '.body | map(.format)'
restish https://api.rest.sh/images --rsh-collect -f '.body | group_by(.format)'
```

## Raw Output

For shell-friendly output, combine a filter with `--rsh-raw`:

```bash
restish https://api.rest.sh/images -f '.body[] | .name' --rsh-raw
```

That prints simple scalar results without JSON quoting and prints arrays of
scalars one item per line.

Example output:

```text
Dragonfly macro
Origami under blacklight
Andy Warhol mural in Miami
Station in Prague
Chihuly glass in boats
```

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

## Common Mistakes

- forgetting the `body.` prefix when you mean the response body in shorthand
- using `jq` aggregation without `--rsh-collect` on paginated endpoints
- expecting filtered output to preserve the original raw response bytes

## Why Filtering Feels Consistent

Because filtering happens after normalization:

- body filters work the same way across formatters
- protocol metadata stays accessible when you need it
- sub-value filtering is predictable in scripts

Once you filter down to a sub-value, Restish renders that filtered value rather
than trying to preserve the original raw response bytes.

## Learn More

- [Output](../output/)
- [Query Syntax](/docs/reference/query-syntax/)
- [Design Records](/docs/contributing/design-records/)

Source material:

- [Design Records](/docs/contributing/design-records/)
