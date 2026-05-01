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
restish https://api.rest.sh/example -f 'body..url'
```

Recursive search and projection are useful when exploring unfamiliar API
responses.

## jq Filters

Restish auto-detects jq-style filters that start with `.` or use jq operators:

{{< restish-example >}}
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name' -o lines
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | map(.format) | unique'
```

Force a language when a filter is ambiguous:

```bash
restish https://api.rest.sh/images --rsh-filter-lang shorthand -f 'body.self'
restish https://api.rest.sh/images --rsh-filter-lang jq -f '.body[] | .self'
```

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
