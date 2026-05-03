---
title: Query Syntax
linkTitle: Query Syntax
weight: 35
description: Reference for shorthand response queries and jq filter selection.
---

Restish filters normalized responses. Use shorthand for paths and projections;
use jq for rich transforms.

## Roots

```bash
restish https://api.rest.sh/ -f headers.Content-Type
restish https://api.rest.sh/images -f links.next
restish https://api.rest.sh/example -f body.basics.profiles
```

## Paths And Indexes

```bash
restish https://api.rest.sh/images -f body[0].name
restish https://api.rest.sh/images -f body[-1].self
restish https://api.rest.sh/images -f 'body[0:2].self'
```

## Selection

```bash
restish https://api.rest.sh/images -f 'body[format = jpeg].name' -o lines
restish https://api.rest.sh/images -f 'body[name.lower contains dragonfly].self' -o lines
```

## Projection And Recursive Search

```bash
restish https://api.rest.sh/example -f 'body.basics.{name, url, profiles}'
restish https://api.rest.sh/images -f '{next: links.next, first: body[0].self}'
restish https://api.rest.sh/example -f 'body..url'
```

## jq

```bash
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name' -o lines
restish https://api.rest.sh/images --rsh-collect -f '.body | map(.format) | unique'
restish https://api.rest.sh/images -f '{next: .links.next, first: .body[0].self}'
restish https://api.rest.sh/example -f '.. | .url?'
```

## Auto-Detection

In `auto` mode, Restish tries both shorthand and jq. If both languages can parse
the expression, bare normalized-response roots such as `links.next` and
`body[0].self` select shorthand. jq expressions use jq's current-input root,
such as `.links.next` and `.body[0].self`.

Recursive descent keeps the same distinction:

```bash
restish https://api.rest.sh/example -f 'body..url'
restish https://api.rest.sh/example -f '.. | .url?'
```

When both languages fail, Restish reports the likely parser first and includes
the other parser's error. Use `--rsh-filter-lang` when you want to force one
language.

## Related Pages

- [Filtering](/docs/guides/filtering/)
- [Shorthand](../shorthand/)
- [Output](../output-formats/)
