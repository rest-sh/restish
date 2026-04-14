---
title: Pagination and Links
linkTitle: Pagination and Links
weight: 70
description: Follow collection pages and inspect hypermedia links in Restish.
---

Restish can automatically parse and follow pagination links across several
hypermedia formats.

## What Restish Recognizes

Restish normalizes links from several sources, including:

- HTTP `Link` headers
- HAL `_links`
- JSON:API top-level `links`
- Siren `links`
- some body-level identifiers such as JSON-LD or TSJ links

Those discovered links are resolved to absolute URLs and exposed consistently
to the rest of the CLI.

## Automatic Pagination

For GET requests, if Restish discovers a `next` link, it can continue fetching
pages automatically.

The default behavior is stream-oriented: items are written as they are found
instead of waiting for every page first.

That makes large listings feel responsive:

```bash
restish https://api.rest.sh/images
```

## Collect Before Filtering

Use `--rsh-collect` when you want all pages gathered into one logical response
before filtering or formatting:

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

This is especially useful for totals, aggregation, and whole-collection table
output.

## Pagination Limits

Restish exposes a few practical safety flags:

- `--rsh-no-paginate` returns only the first page
- `--rsh-max-pages` bounds how many pages will be fetched
- `--rsh-max-items` bounds how many items are emitted or collected

Examples:

```bash
restish https://api.rest.sh/images --rsh-no-paginate
restish https://api.rest.sh/images --rsh-max-pages 3
restish https://api.rest.sh/images --rsh-max-items 250
```

## APIs With Nested Collections

Some APIs do not return the item array at the top level. Restish can be guided
with API config:

```json
{
  "apis": {
    "myapi": {
      "pagination": {
        "items_path": "data",
        "next_path": "links.next"
      }
    }
  }
}
```

That tells Restish where to find the collection items and the next page URL in
the response body.

## Inspect Links Explicitly

Pagination is built on the same normalized links model used elsewhere in the
CLI. When you want to inspect what links Restish found, use the links-focused
commands and filters.

Examples:

```bash
restish https://api.rest.sh/images -f links
restish https://api.rest.sh/images -f links.next -r
```

## When To Stream vs Collect

Stream when:

- the result set may be large
- you want first output quickly
- you are piping items onward one at a time

Collect when:

- your filter needs the whole result set
- you want to count or aggregate
- you want one final formatted document

## Related Guides

- [Filtering](../filtering/)
- [Requests](../requests/)

Source material:

- [Design Records](/docs/contributing/design-records/)
