---
title: Links and Hypermedia
linkTitle: Links and Hypermedia
weight: 75
description: Inspect normalized links and understand how Restish follows paginated APIs.
---

Restish extracts links from responses and normalizes them into a simple
relation-to-URL map. That is why pagination and link inspection work across
several API styles instead of only one.

## Built-In Link Sources

Restish currently recognizes:

- HTTP `Link` headers
- HAL `_links`
- JSON:API top-level `links`
- Siren `links`
- JSON-LD and TSJ `@id`

All discovered links are resolved to absolute URLs before Restish stores them.

## Inspect Links Directly

Use the `links` command to fetch a resource and print its discovered links:

```bash
restish links https://api.rest.sh/images
restish links https://api.rest.sh/images next
restish links https://api.rest.sh/images self next prev
```

## Links In Filters

Because links live in the normalized response, you can also access them through
filters:

```bash
restish https://api.rest.sh/images -f links.next -r
restish https://api.rest.sh/images -f links.self -r
```

## Relationship To Pagination

For `GET` requests, Restish can use the normalized `next` link to continue
fetching pages automatically.

When an API puts pagination state in the body instead of a link header, use
per-API pagination config to teach Restish where to look.

## Related Guides

- [Pagination](/docs/guides/pagination/)
- [Filtering](/docs/guides/filtering/)
- [Inspect Response Links Recipe](/docs/recipes/inspect-response-links/)
