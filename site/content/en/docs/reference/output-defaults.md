---
title: Output Defaults
linkTitle: Output Defaults
weight: 31
description: Reference for default output choices on terminals, redirects, filters, pagination, and streams.
---

Restish chooses output defaults from context. Scripts should still pass `-o`
when the expected shape matters.

## Main Rule

| Context | Default |
| --- | --- |
| Interactive terminal, unfiltered structured response | `readable` |
| Redirected stdout, unfiltered response | response body bytes |
| Explicit filter selecting a scalar | plain scalar text |
| Explicit filter selecting structured data | JSON-shaped readable output on a terminal, JSON when redirected |
| Paginated items with no collect mode | stream item results as they arrive |
| Live streams | record-oriented output; use `ndjson` or `lines` for scripts; `-o json` is rejected |

## Examples

```bash
restish api.rest.sh/images
restish api.rest.sh/images -o json > images.json
restish api.rest.sh/images/jpeg > dragonfly.jpg
restish api.rest.sh/content/cbor > response.cbor
restish api.rest.sh/content/cbor -o json > response.json
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
```

## Filtering

Filters run before formatting:

```bash
restish api.rest.sh/images -f body.self -o lines
```

Without `--rsh-collect`, paginated filters run per item. With
`--rsh-collect`, Restish builds the logical collection first, then filters.

## Raw Bytes

`-r` or `--rsh-raw` requests response body bytes explicitly. Raw mode cannot be
combined with filters because filters operate on decoded normalized responses.

## Metadata Shortcuts

`--rsh-headers` is shorthand for `-f headers`. `--rsh-status` is shorthand for
`-f status`.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Filtering](/docs/guides/filtering/)
