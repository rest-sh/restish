---
title: Pagination and Links
linkTitle: Pagination and Links
weight: 50
description: Follow collection pages, inspect links, and choose streaming or collection behavior.
---

Restish follows recognized `next` links for collection responses by default.
Use limits and collect mode to make the behavior explicit.

## Automatic Pagination

{{< restish-example >}}
restish https://api.rest.sh/images -f body.self -o lines
{{< /restish-example >}}

When a response exposes a `next` link, Restish follows it until there are no
more pages or a configured limit is reached.
Next-page URLs must stay on the same origin: scheme, hostname, and effective
port must match. Pagination stops with a warning before following a link that
changes any of those origin components.

## Inspect One Page

{{< restish-example >}}
restish https://api.rest.sh/images --rsh-no-paginate -f links.next
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images --rsh-no-paginate
```

Use this when you want to understand the server's paging model before collecting
more data.

## Limit Pagination

{{< restish-example >}}
restish https://api.rest.sh/images --rsh-max-items 100
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images --rsh-max-pages 3
```

`--rsh-max-pages` protects you from unexpectedly large collections. It defaults
to `25`; pass `--rsh-max-pages 0` for unlimited pagination. When the cap is
reached, Restish prints a warning and exits successfully with the pages already
processed.
`--rsh-max-items` bounds the total logical items Restish processes.
If a later page returns an HTTP error, Restish stops and returns that status
instead of formatting a partial collection as if it were complete.
Configured `pagination.items_path` and `pagination.next_path` are strict: a
typo, missing configured field, wrong body type, or non-string `next_path`
result returns an error instead of silently truncating the collection.

## Collect Before Filtering

Without `--rsh-collect`, filters run once for each paginated item. Output
format does not change that scope: `-o ndjson` can write filtered records as
they arrive, while `-o json` gathers the filtered item results into one valid
JSON document. Some filters need the whole collection:

{{< restish-example >}}
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
{{< /restish-example >}}

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | map(.self)'
```

Without `--rsh-collect`, item-oriented output can start sooner and use less
memory.

## Links Command

{{< restish-example >}}
restish links https://api.rest.sh/images next
{{< /restish-example >}}

```bash
restish links https://api.rest.sh/images
```

The `links` command is useful when you only want the normalized hypermedia link
map and not the response body.

## Related Pages

- [Links and Hypermedia](../links-and-hypermedia/)
- [Links Command](/docs/reference/links-command/)
- [Output](../output/)
- [Count Items Across All Pages](/docs/recipes/count-items-across-all-pages/)
