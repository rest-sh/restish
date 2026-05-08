---
title: Output Formats
linkTitle: Output Formats
weight: 30
description: Reference for Restish output formats and document-versus-record behavior.
---

Output formats decide how a normalized response is rendered after decoding,
pagination or streaming, and filtering. They do not change request `Accept`
headers or server-side content negotiation.

## Document Formats

Document formats produce one coherent result:

| Format | Use |
| --- | --- |
| `readable` | Human-oriented terminal output. |
| `json` | One JSON document. |
| `yaml` | One YAML document. |
| `cbor` | One CBOR document. |
| `gron` | Greppable path/value assignments. |
| `image` | Terminal image rendering for image responses. |

```bash
restish api.rest.sh/images -o readable
restish api.rest.sh/images --rsh-collect -o json
restish api.rest.sh/images -o yaml
restish api.rest.sh/example -o gron
restish api.rest.sh/images/png -o image
```

For redirects, unfiltered responses already write body bytes:

```bash
restish api.rest.sh/images/jpeg > dragonfly.jpg
restish api.rest.sh/content/cbor > response.cbor
restish api.rest.sh/content/cbor -o json > response.json
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

`image` renders image responses in capable terminals. Redirect the same request
when you want a file instead.

```bash
restish api.rest.sh/images/png -o image
```

## Filters And Scalar Lines

```bash
restish api.rest.sh/images -f body.name -o lines
```

Explicit scalar filters print without JSON string quotes. `-o lines` accepts
arrays and streams of scalar values and rejects structured objects.

## Raw Output

Raw output is a mode, not an `-o` format:

```bash
restish api.rest.sh/bytes/64 --rsh-raw > sample.bin
```

Raw mode writes response body bytes and cannot combine with filters.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Defaults](../output-defaults/)
- [Filtering](/docs/guides/filtering/)
