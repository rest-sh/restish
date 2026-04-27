---
title: Links and Hypermedia
linkTitle: Links and Hypermedia
weight: 55
description: Inspect normalized links from headers and response bodies.
---

Restish normalizes hypermedia links so filters, pagination, and the `links`
command can use one shape even when APIs expose links differently.

## Recognized Link Sources

Restish can extract links from sources such as:

- HTTP `Link` headers
- common body link fields
- HAL-style `_links`
- JSON:API-style `links`
- Siren and related REST-ish representations

The exact source matters less to users than the normalized result: relation
names such as `self`, `next`, and `prev` become queryable under `links`.

## Inspect Links

```bash
restish links https://api.rest.sh/images
restish links https://api.rest.sh/images next self
```

Filter links from a normal request:

```bash
restish https://api.rest.sh/images -f links.next -r
restish https://api.rest.sh/images -f links.self -r
```

## Pagination Uses Links

Automatic pagination follows normalized `next` links. Disable it when you want
to inspect only the first page:

```bash
restish https://api.rest.sh/images --rsh-no-paginate -f links
```

## Related Pages

- [Pagination and Links](../pagination/)
- [Links Command](/docs/reference/links-command/)
- [Output](/docs/guides/output/)
- [Example API](/docs/reference/example-api/)
