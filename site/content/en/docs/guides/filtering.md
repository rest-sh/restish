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
restish https://api.rest.sh/ -f headers.Content-Type -r
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images -f links.next -r
restish https://api.rest.sh/example -f body.basics.profiles
```

## Shorthand Paths

{{< restish-example >}}
restish https://api.rest.sh/images -f body[0].name -r
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images -f body[-1].self -r
restish https://api.rest.sh/example -f body.volunteer[0].organization -r
```

## Selection And Projection

{{< restish-example >}}
restish https://api.rest.sh/images -f 'body[format = jpeg].self' -r
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
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name' -r
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

## Raw Scalars

{{< restish-example >}}
restish https://api.rest.sh/images -f '.body[] | .name' -r
{{< /restish-example >}}

Raw mode prints scalar results without JSON string quotes and prints array items
one per line.

## Related Pages

- [Query Syntax](/docs/reference/query-syntax/)
- [Output](../output/)
- [Pagination](../pagination/)
- [Filter Response Fields](/docs/recipes/filter-response-fields/)
