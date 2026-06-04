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
| `toon` | Token-dense encoding for feeding responses to LLM agents. |
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

## TOON For Agents

[TOON](https://github.com/toon-format/spec) (Token-Oriented Object Notation) is
a compact, indentation-based encoding of the JSON data model. It is built for
feeding API responses to LLM agents — for example when Restish is the tool
surface an agent calls instead of an MCP server — where every response token
has a cost.

TOON's savings come from uniform arrays of objects. Such an array collapses
into a table that declares its field names once and then streams one row per
record, instead of repeating every key on every element:

```bash
restish api.rest.sh/images -o toon
```

```text
[5]{format,name,self}:
  jpeg,Dragonfly macro,/images/jpeg
  webp,Origami under blacklight,/images/webp
  gif,Andy Warhol mural in Miami,/images/gif
  png,Station in Prague,/images/png
  heic,Chihuly glass in boats,/images/heic
```

Pair `-o toon` with a filter that projects the response down to the records and
fields you care about. Filtering drops unneeded fields entirely and usually
saves more tokens than re-encoding alone, and projecting to a uniform list is
what keeps the tabular form:

```bash
restish api.rest.sh/images -f '.[] | {name, format}' -o toon
```

```text
[5]{format,name}:
  jpeg,Dragonfly macro
  webp,Origami under blacklight
  gif,Andy Warhol mural in Miami
  png,Station in Prague
  heic,Chihuly glass in boats
```

For paginated list endpoints, add `--rsh-collect` so every page is gathered into
one array and rendered as a single table. Without it, each page's items render
as separate documents and the tabular savings are lost:

```bash
restish api.rest.sh/images --rsh-collect -o toon
```

### How TOON compares

Tokens for the same data rendered in each format, counted with `o200k_base`
(GPT-4o-class). "Uniform 100" is a 100-row record collection; "Nested 40" is a
collection of nested, irregular objects.

| Format | Uniform 100 | Nested 40 |
| --- | --: | --: |
| **toon** | **1,689** | **3,343** |
| json (compact) | 2,903 | 2,762 |
| json (pretty) | 5,002 | 4,842 |
| yaml | 3,700 | 3,360 |
| ndjson | 3,000 | 2,800 |
| gron | 6,403 | 6,783 |

On uniform record collections TOON is the most token-efficient text format, and
the lead grows with row count. On nested or irregular data, compact JSON and
NDJSON edge it out — TOON's per-line indentation outweighs the savings — so
project to a uniform list first, or stay on JSON.

Not shown: `table` is human-only and truncates long values (lossy), `lines`
only handles scalar arrays, and `cbor` is binary rather than text.

Other tradeoffs:

- TOON is output-only. Restish does not accept TOON request bodies.
- Token savings only pay off if your model parses TOON as reliably as JSON.
  Validate against your own model and data before relying on it.

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
