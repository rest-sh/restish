---
title: Links and Hypermedia
linkTitle: Links and Hypermedia
weight: 55
description: Inspect normalized links from headers and response bodies.
aliases:
  - /docs/recipes/inspect-response-links/
  - /docs/recipes/show-links-for-one-relation/
---

Restish normalizes hypermedia links so filters, pagination, and the `links`
command can use one shape even when APIs expose links differently.

## Recognized Link Sources

Restish can extract links from sources such as:

- HTTP `Link` headers
- HAL-style `_links`, including arrays of HAL resources
- JSON:API-style top-level `links` and resource `links.self`
- Siren links
- JSON-LD or TSJ `@id`
- simple REST-ish `self` fields on top-level objects or nested array items

The exact source matters less to users than the normalized result: relation
names such as `self`, `next`, and `prev` become queryable under `links`.
Nested array item links use relation names based on the field, such as
`things-item`.

## Inspect Links

{{< restish-example >}}
restish links api.rest.sh/images
{{< /restish-example >}}

```bash
restish links api.rest.sh/images next self
```

Filter links from a normal request:

{{< restish-example >}}
restish api.rest.sh/images -f links.next
{{< /restish-example >}}

```bash
restish api.rest.sh/images -f links.self
```

## Pagination Uses Links

Automatic pagination follows normalized `next` links. Disable it when you want
to inspect only the first page:

{{< restish-example >}}
restish api.rest.sh/images --rsh-no-paginate -f links
{{< /restish-example >}}

## Related Pages

- [Pagination and Links](../pagination/)
- [Commands](/docs/reference/commands/)
- [Output](/docs/guides/output/)
- [Example API](/docs/reference/example-api/)
