---
title: Filtering
linkTitle: Filtering
weight: 45
description: Select headers, links, and body fields with shorthand queries or jq filters.
---

Filtering trims a normalized Restish response before formatting. Use shorthand
for direct paths and projections; use jq for richer transforms.

## Filter Roots

- `headers` for response headers
- `links` for normalized hypermedia links
- `body` for decoded response body

{{< restish-example >}}
restish https://api.rest.sh/ -f headers.Content-Type
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images -f links.next
restish https://api.rest.sh/example -f body.basics.profiles
```

## Shorthand Paths

{{< restish-example >}}
restish https://api.rest.sh/images -f body[0].name
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images -f body[-1].self
restish https://api.rest.sh/example -f body.volunteer[0].organization
```

## Selection And Projection

{{< restish-example >}}
restish https://api.rest.sh/images -f 'body[format = jpeg].self' -o lines
{{< /restish-example >}}

```bash
restish https://api.rest.sh/example -f 'body.basics.{name, url, profiles}'
restish https://api.rest.sh/images -f '{next: links.next, first: body[0].self}'
restish https://api.rest.sh/example -f 'body..url'
```

Recursive search and projection are useful when exploring unfamiliar API
responses.

## jq Filters

jq filters use jq's current-input root, operators, functions, and pipeline
syntax:

{{< restish-example >}}
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name' -o lines
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | map(.format) | unique'
restish https://api.rest.sh/images -f '{next: .links.next, first: .body[0].self}'
restish https://api.rest.sh/example -f '.. | .url?'
```

Force a language when a filter is ambiguous:

```bash
restish https://api.rest.sh/images --rsh-filter-lang shorthand -f '{next: links.next}'
restish https://api.rest.sh/images --rsh-filter-lang jq -f '{next: .links.next}'
```

In the default `auto` mode, Restish tries both shorthand and jq. If both
languages can parse the filter, bare normalized-response roots such as
`links.next` mean shorthand and jq's current-input root such as `.links.next`
means jq. Recursive descent follows the same rule: `..url` is shorthand, while
`.. | .url?` is jq. When both languages fail, Restish reports the likely parser
first and still includes the other parser's error.

## Pagination And Collecting

Default pagination streams items as they arrive. Use `--rsh-collect` when the
filter needs the whole collection:

{{< restish-example >}}
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
{{< /restish-example >}}

## Scalar Lines

{{< restish-example >}}
restish https://api.rest.sh/images -f '.body[] | .name' -o lines
{{< /restish-example >}}

Explicit scalar filters print without JSON string quotes. Use `-o lines` when
the filtered value is an array or stream of scalars and you want one value per
line. `-o lines` rejects structured objects; use `-o json` when you need to
preserve array or object shape.

## Related Pages

- [Query Syntax](/docs/reference/query-syntax/)
- [Output](../output/)
- [Pagination](../pagination/)
- [Filter Response Fields](/docs/recipes/filter-response-fields/)
