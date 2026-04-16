---
title: Example API
linkTitle: Example API
weight: 15
description: Canonical api.rest.sh endpoints and commands used throughout the Restish docs.
---

The docs use `https://api.rest.sh` as the main example API whenever a concrete,
working endpoint makes the explanation clearer.

That keeps examples consistent across getting started pages, guides, recipes,
and command references.

## Configure It Once

If you want the shortest commands, register the API under a short name:

```bash
restish api configure example https://api.rest.sh
```

From there, both generic and API-aware examples in the docs make sense:

```bash
restish https://api.rest.sh/
restish example list-images
restish example get-image jpeg
```

## Canonical Endpoints

- `https://api.rest.sh/` for first requests, headers, and simple generic GET examples.
- `https://api.rest.sh/images` for pagination, links, table output, and list filtering examples.
- `https://api.rest.sh/images/<format>` for item-level and raw/binary output examples such as `jpeg`.
- `https://api.rest.sh/example` for nested response filtering examples.
- `https://api.rest.sh/types` for shorthand, editing, and schema-oriented examples.
- `https://api.rest.sh/books` for bulk checkout examples carried over from the older docs.

## Where Each Endpoint Shows Up

| Endpoint | Used In |
| --- | --- |
| `https://api.rest.sh/` | first request, header inspection, basic generic GET examples |
| `https://api.rest.sh/images` | pagination, links, filtering, table output, NDJSON output |
| `https://api.rest.sh/images/<format>` | raw downloads, image rendering, item lookup |
| `https://api.rest.sh/example` | nested filtering and projection examples |
| `https://api.rest.sh/types` | edit workflow, shorthand-oriented writable examples |
| `https://api.rest.sh/books` | bulk-management examples |

## Why These Resources Show Up Repeatedly

These endpoints cover the main workflows Restish is designed around:

- a root resource for generic requests
- a collection with links for pagination and hypermedia
- nested data for filtering and projection
- structured writable examples for shorthand input
- a books collection for bulk workflows

## Related Pages

- [First Request](/docs/getting-started/first-request/)
- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [Requests](/docs/guides/requests/)
- [Filtering](/docs/guides/filtering/)
- [Pagination and Links](/docs/guides/pagination/)
- [Bulk Management](/docs/guides/bulk-management/)
