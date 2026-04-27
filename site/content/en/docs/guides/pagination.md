---
title: Pagination and Links
linkTitle: Pagination and Links
weight: 50
description: Follow collection pages, inspect links, and choose streaming or collection behavior.
---

Restish follows recognized `next` links for collection responses by default.
Use limits and collect mode to make the behavior explicit.

## Automatic Pagination

```bash
restish https://api.rest.sh/images -f body.self -r
```

When a response exposes a `next` link, Restish follows it until there are no
more pages or a configured limit is reached.

## Inspect One Page

```bash
restish https://api.rest.sh/images --rsh-no-paginate
restish https://api.rest.sh/images --rsh-no-paginate -f links.next -r
```

Use this when you want to understand the server's paging model before collecting
more data.

## Limit Pagination

```bash
restish https://api.rest.sh/images --rsh-max-pages 3
restish https://api.rest.sh/images --rsh-max-items 100
```

`--rsh-max-pages` protects you from unexpectedly large collections.
`--rsh-max-items` bounds the total logical items Restish processes.

## Collect Before Filtering

Some filters need the whole collection:

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
restish https://api.rest.sh/images --rsh-collect -f '.body | map(.self)'
```

Without `--rsh-collect`, item-oriented output can start sooner and use less
memory.

## Links Command

```bash
restish links https://api.rest.sh/images
restish links https://api.rest.sh/images next
```

The `links` command is useful when you only want the normalized hypermedia link
map and not the response body.

## Related Pages

- [Links and Hypermedia](../links-and-hypermedia/)
- [Links Command](/docs/reference/links-command/)
- [Output](../output/)
- [Count Items Across All Pages](/docs/recipes/count-items-across-all-pages/)
