---
title: Query Syntax
linkTitle: Query Syntax
weight: 35
description: Reference for shorthand response queries, normalized response roots, jq selection, and filter scope.
---

Restish filters the normalized response before formatting. Use shorthand for
direct paths, projections, recursive search, and simple selection. Use jq for
larger transforms.

## Roots

Most filters start from one of these normalized roots:

| Root | Meaning |
| --- | --- |
| `status` | Numeric HTTP status code. |
| `headers` | First value for each response header. |
| `headers_all` | Complete header map for repeated values. Use quoted shorthand such as `headers_all."Set-Cookie"[0]` or jq syntax such as `.headers_all["Set-Cookie"]` when you need every value. |
| `links` | Normalized hypermedia links. |
| `body` | Decoded response body. |
| `proto` | HTTP protocol string such as `HTTP/2.0`. |

```bash
restish api.rest.sh/ -f headers.Content-Type
restish api.rest.sh/ -f 'headers_all."Set-Cookie"[0]'
restish api.rest.sh/images -f links.next
restish api.rest.sh/example -f body.basics.profiles
```

## Paths And Indexes

Use dots for object fields and brackets for array positions:

```bash
restish api.rest.sh/images --rsh-collect -f body[0].name
restish api.rest.sh/images --rsh-collect -f body[-1].self
restish api.rest.sh/images --rsh-collect -f 'body[0:2].self'
```

Negative indexes count from the end. Slices need the whole collection, so use
`--rsh-collect` for paginated arrays. Slice bounds are inclusive:
`body[0:2]` selects indexes `0`, `1`, and `2`.

## Selection

Selection filters array items:

```bash
restish api.rest.sh/images --rsh-collect -f 'body[format == jpeg].name' -o lines
restish api.rest.sh/images --rsh-collect -f 'body[name.lower contains dragonfly].self' -o lines
```

Common operators include equality and string containment. Modifiers such as
`.lower` help with case-insensitive matching.

## Projection

Projection keeps only selected fields:

```bash
restish api.rest.sh/example -f 'body.basics.{name, url, profiles}'
restish api.rest.sh/images --rsh-no-paginate -f '{next: links.next, first: body[0].self}'
```

Projection is useful when the next command or reader needs a smaller object
instead of a scalar field.

## Recursive Search

Shorthand recursive descent searches below a path:

```bash
restish api.rest.sh/example -f 'body..url'
restish api.rest.sh/example -f '..url|[@ contains github]'
```

Use recursive search while exploring unfamiliar responses. Prefer exact paths
once a script depends on a stable shape.

## jq

jq filters use jq's current-input root:

```bash
restish api.rest.sh/images --rsh-collect -f '.body[] | select(.format == "jpeg") | .name' -o lines
restish api.rest.sh/images --rsh-collect -f '.body | map(.format) | unique'
restish api.rest.sh/images --rsh-no-paginate -f '{next: .links.next, first: .body[0].self}'
restish api.rest.sh/ -f '.headers_all["Content-Type"]'
restish api.rest.sh/example -f '.. | .url?'
```

Use jq when you need functions, larger transformations, joins, reductions, or
other query-program behavior.

## Auto-Detection

The default filter language is `auto`. Restish tries shorthand and jq, then
chooses the expression that best matches the input:

- Bare roots such as `links.next` and `body[0].self` mean shorthand.
- A leading jq current-input field such as `.links.next` means jq.
- `body..url` is shorthand recursive descent.
- `.. | .url?` is jq recursive descent.
- If both fail, Restish reports the likely parser first and includes the other
  parser error.

Force a language when ambiguity would surprise readers:

```bash
restish api.rest.sh/images --rsh-filter-lang shorthand -f '{next: links.next}'
restish api.rest.sh/images --rsh-filter-lang jq -f '{next: .links.next}'
```

## Pagination Scope

By default, paginated responses stream items as pages arrive and the filter runs
per item. Each item is presented as `{"body": <item>}`, so select item fields
with `body.self` or jq `.body.self`. Use `--rsh-collect` when the filter needs
the whole collection:

```bash
restish api.rest.sh/images -f body.self -o lines
restish api.rest.sh/images --rsh-collect -f '.body | length'
```

Collect mode waits for all pages and holds the logical collection in memory.
Use item-by-item filters when you want early output or large-result safety.

## Scalar Output

Explicit scalar filters print without JSON string quotes in auto output.
Use `-o lines` when a selected array or stream should become one scalar value
per line:

```bash
restish api.rest.sh/images -f body.name -o lines
```

Use `-o json` when the selected value should stay a JSON value for another
program.

## Related Pages

- [Filtering](/docs/guides/filtering/)
- [Output](/docs/guides/output/)
- [Shorthand](../shorthand/)
- [Output Formats](../output-formats/)
