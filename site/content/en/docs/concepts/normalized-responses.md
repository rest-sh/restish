---
title: Normalized Responses
linkTitle: Normalized Responses
weight: 45
description: Understand the response shape Restish uses for filtering, links, pagination, and output.
---

Restish does not send raw HTTP responses directly to every output formatter. It
first normalizes the response into a stable shape. That makes filtering, links,
pagination, streaming, and formatter plugins compose predictably.

## Main Roots

Most filters start from one of these roots:

- `status`: HTTP status and protocol information
- `headers`: response headers
- `links`: normalized hypermedia links from headers or body formats
- `body`: decoded response body

Examples:

```bash
restish https://api.rest.sh/ -f headers.Content-Type -r
restish https://api.rest.sh/images -f links.next -r
restish https://api.rest.sh/example -f body.basics.profiles
```

## Why It Matters

Because response data is normalized first, the same habits work across many
features:

- output formats can render decoded bodies or full responses
- filters can select headers, links, and nested body fields
- pagination can follow normalized `next` links
- streaming can process one event at a time
- plugins can format the same response model the host uses

## Documents And Records

Document formats such as `json`, `yaml`, and `readable` should produce one
coherent result. Record formats such as `ndjson` can emit one item or event at a
time. This distinction is important for pagination and live streams.

```bash
restish https://api.rest.sh/images --rsh-collect -o json
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

## Related Pages

- [Output](/docs/guides/output/)
- [Filtering](/docs/guides/filtering/)
- [Output Defaults](/docs/reference/output-defaults/)
- [Output Formats](/docs/reference/output-formats/)
