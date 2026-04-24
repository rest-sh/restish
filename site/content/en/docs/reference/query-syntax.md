---
title: Query Syntax
linkTitle: Query Syntax
weight: 31
description: Reference for the shorthand query language and jq, including how Restish chooses between them.
---

Restish supports two filter languages:

- shorthand query syntax for direct paths, selection, filtering, and projection
- `jq` for richer transformation

This page documents the shorthand query side in more depth and explains how
Restish decides when a filter expression is shorthand versus `jq`.

## Restish Filter Detection

Restish treats a filter as shorthand when it starts with one of the recognized
normalized response roots:

- `body`
- `headers`
- `links`
- `status`
- `proto`
- `@`

Otherwise Restish treats the filter as `jq`.

Examples:

```bash
restish https://api.rest.sh/example -f body.basics.profiles
restish https://api.rest.sh/ -f headers.Content-Type
restish https://api.rest.sh/images -f links.next
restish https://api.rest.sh/images -f '.body[] | .name'
```

The first three are shorthand. The last one is `jq`.

## Query Syntax Diagram

The upstream shorthand project ships this query syntax diagram:

![Query syntax diagram](/images/query-syntax.svg)

Inside filter brackets, the `filter` portion is defined by the upstream
[`mexpr`](https://github.com/danielgtaylor/mexpr) expression language.

## Core Path Syntax

The shorthand query language supports:

- object paths such as `foo.items.name`
- wildcards such as `foo.*.name`
- array indexing and slicing such as `foo.items[1:2].name`
- negative indexes such as `foo.items[-1].name`
- array filtering such as `foo.items[name.lower startsWith d]`
- object field selection such as `foo.{created, names: items.name}`
- recursive search such as `foo..name`
- pipes to stop processing at a point with `|`
- array flattening with `[]`

In Restish, those paths operate against the normalized response object, so you
usually start at `body`, `headers`, or `links`.

## Direct Path Examples

Get one field:

```bash
restish https://api.rest.sh/example -f body.basics.profiles
```

Get one header:

```bash
restish https://api.rest.sh/ -f headers.Content-Type
```

Get one link relation:

```bash
restish https://api.rest.sh/images -f links.next
```

## Wildcards, Indexes, And Slices

Wildcard property lookup:

```bash
restish https://api.rest.sh/example -f 'body.*.profiles'
```

Array index:

```bash
restish https://api.rest.sh/images -f 'body[0].name'
```

Last item with a negative index:

```bash
restish https://api.rest.sh/images -f 'body[-1].name'
```

Slice:

```bash
restish https://api.rest.sh/images -f 'body[0:2].name'
```

## Filtering

Bracket filters select matching items from an array using the shorthand filter
language:

```bash
restish https://api.rest.sh/images -f 'body[format = jpeg].name'
restish https://api.rest.sh/images -f 'body[name.lower startsWith d].self'
```

This is different from `jq`. The filter expression inside `[...]` is shorthand
query filtering, not a `jq` program.

## Field Selection And Projection

Select a smaller object:

```bash
restish https://api.rest.sh/example -f 'body.basics.{profiles, active}'
```

Rename a selected field:

```bash
restish https://api.rest.sh/images -f 'body.{path: self, kind: format}'
```

These are especially useful when paired with `-o json`, `-o yaml`, or `-o csv`.

## Recursive Search And Flattening

Recursive lookup:

```bash
restish https://api.rest.sh/example -f 'body..profiles'
```

Flatten nested arrays:

```bash
restish https://api.rest.sh/example -f 'body.people[].emails[]'
```

## Pipes

The shorthand query language includes `|` to stop processing at a point in the
expression. This is part of shorthand query syntax, not shell piping.

```bash
restish https://api.rest.sh/images -f 'body | [0:2]'
```

If you need the more powerful transformation model most people associate with
pipes, use `jq` instead:

```bash
restish https://api.rest.sh/images -f '.body[] | .name'
```

## When To Use Shorthand Vs jq

Use shorthand when:

- you want one path or one relation
- you want one header value
- you want lightweight filtering or field projection
- you want syntax that matches the upstream shorthand patch/query model

Use `jq` when:

- you need arbitrary transformation
- you want reducers, grouping, or custom functions
- the expression naturally starts with `.` anyway

## Related Pages

- [Filtering](/docs/guides/filtering/)
- [Shorthand Syntax](../shorthand/)
- [Global Flags](../global-flags/)
- [Upstream shorthand README](https://github.com/danielgtaylor/shorthand/blob/main/README.md)
