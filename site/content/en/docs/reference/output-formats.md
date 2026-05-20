---
title: Output Formats
linkTitle: Output Formats
weight: 30
description: Reference for Restish output formats and document-versus-record behavior.
---

Output formats decide how the selected body/value is rendered after decoding,
pagination or streaming, and filtering. They do not print HTTP status or
headers; use `--rsh-print` for those exchange parts. They also do not change
request `Accept` headers or server-side content negotiation. Redirected
rendered document output is pretty by default; pass `--rsh-print=b` for compact
rendered JSON.

## Document Formats

Document formats produce one coherent result:

| Format | Use |
| --- | --- |
| `auto` | Default body/value presentation. |
| `json` | One JSON document. |
| `yaml` | One YAML document. |
| `cbor` | One CBOR document. |
| `gron` | Greppable path/value assignments. |
| `table` | Terminal table output for records or object collections. |
| `image` | Terminal image rendering for image responses. |

```bash
restish api.rest.sh/images -o auto
restish api.rest.sh/images --rsh-collect -o json
restish api.rest.sh/images -o yaml
restish api.rest.sh/example -o gron
restish api.rest.sh/images/png
```

For redirects, unfiltered responses already write body bytes:

```bash
restish api.rest.sh/images/jpeg > dragonfly.jpg
restish api.rest.sh/formats/cbor > response.cbor
restish api.rest.sh/formats/cbor -o json > response.json
```

## Record Formats

Record formats emit one item or event at a time:

| Format | Use |
| --- | --- |
| `ndjson` | One JSON value per line. |
| `lines` | One scalar value per line without JSON string quotes. |
| plugin formats such as `csv` | Formatter-specific record output. |

```bash
restish api.rest.sh/images -o ndjson -f body.self
restish api.rest.sh/images -f body.self -o lines
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
```

Use record formats for large paginated responses, live streams, and shell
loops.

## Tables

Tables are for terminal inspection, not stable machine parsing:

```bash
restish api.rest.sh/images -o table --rsh-columns name,format,self
restish api.rest.sh/images -o table --rsh-sort-by name
```

## Images

The `auto` default renders `image/*` responses in capable interactive
terminals. Use `-o image` to force image rendering when the default would be
something else. Redirect the same request when you want a file instead.

```bash
restish api.rest.sh/images/png
```

## Filters And Scalar Lines

```bash
restish api.rest.sh/images -f body.name -o lines
```

Explicit scalar filters print without JSON string quotes. `-o lines` accepts
arrays and streams of scalar values and rejects structured objects.

## Raw Bytes

Raw byte output is automatic for redirected unfiltered responses. It is not an
`-o` format and not a `--rsh-print` part:

```bash
restish api.rest.sh/bytes/64 > sample.bin
restish api.rest.sh/images/jpeg > dragonfly.jpg
```

Choose a format such as `-o json`, `-o lines`, or `-o table` when you want
Restish to render a decoded value instead.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Defaults](../output-defaults/)
- [Filtering](/docs/guides/filtering/)
