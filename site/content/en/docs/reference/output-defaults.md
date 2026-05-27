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
| Interactive terminal, unfiltered response | response status, headers, and formatted body on stdout |
| Redirected stdout, unfiltered response with no explicit output transform | response body bytes |
| Explicit filter selecting a scalar | plain scalar text |
| Explicit filter selecting structured data | pretty JSON-shaped auto output |
| Paginated items with no collect mode | stream item results as they arrive |
| Live streams | record-oriented output; use `ndjson` or `lines` for scripts; plain `-o json` requires `--rsh-collect` and finite `--rsh-max-items` |

## Examples

```bash
restish api.rest.sh/images
restish api.rest.sh/images -o json > images.json
restish api.rest.sh/images/jpeg > dragonfly.jpg
restish api.rest.sh/formats/cbor > response.cbor
restish api.rest.sh/formats/cbor -o json > response.json
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
```

Use `--rsh-print=b` when a script wants compact rendered JSON instead of the
pretty default for transformed output:

```bash
restish api.rest.sh/types --rsh-print=b > types.json
restish api.rest.sh/types -f body.object --rsh-print=b > object.json
```

## Filtering

Filters run before formatting:

```bash
restish api.rest.sh/images -f body.self -o lines
```

Without `--rsh-collect`, paginated filters run per item. With
`--rsh-collect`, Restish builds the logical collection first, then filters.

## Raw Bytes

With the default `--rsh-print=auto`, redirected unfiltered responses preserve
the original response body bytes. That is the raw-byte path for downloads and
binary-safe pipelines; do not choose a filter, metadata shortcut, collection, or
`-o` format when you want the payload unchanged. Response middleware plugins do
not run on this path, so installed plugins cannot silently rewrite saved files.
This does not change server-side content negotiation: Restish's default
`Accept` header prefers JSON and other text-friendly structured formats, and
you can request CBOR, MessagePack, Ion, or another format explicitly with `-H`
or a profile.

## Metadata Shortcuts

`--rsh-headers` is shorthand for `-f headers`. `--rsh-status` is shorthand for
`-f status`.

## Print Parts

`--rsh-print` controls which HTTP exchange parts go to stdout. `auto` prints
`hbpc` on a terminal for ordinary unfiltered responses. When stdout is
redirected and there is no filter, metadata shortcut, collection, or explicit
`-o` format, `auto` writes response body bytes and skips response middleware.
Filters, metadata shortcuts, collection, and explicit output formats print `bp`
by default.

The letters are `H` request headers, `B` request body, `h` response status and
headers, `b` rendered body, `p` pretty formatting, and `c` color. Sensitive
headers are redacted in printed request/response headers. Omit `p` by passing
`--rsh-print=b` when compact rendered output matters.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Filtering](/docs/guides/filtering/)
