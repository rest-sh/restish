---
title: Output Formats
linkTitle: Output Formats
weight: 30
description: Reference for Restish output formats and document-versus-record behavior.
---

Output formats decide how a normalized response is rendered after decoding,
pagination or streaming, and filtering. They do not change paginated filter
scope: without `--rsh-collect`, filters run once per item.

## Document Formats

Document formats produce one coherent result:

- `readable`: human-oriented terminal output
- `json`: one JSON document
- `yaml`: one YAML document
- `cbor`: one CBOR document
- `image`: terminal image rendering for image responses
- `gron`: one greppable path/value document

Examples:

```bash
restish https://api.rest.sh/images -o readable
restish https://api.rest.sh/images --rsh-collect -o json
restish https://api.rest.sh/images -o yaml
restish https://api.rest.sh/example -o gron
restish https://api.rest.sh/images/png -o image
```

For file redirects, binary responses already write body bytes. This includes
`image/*`, `application/octet-stream`, `application/zip`, and unknown
non-text payloads:

```bash
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
restish https://api.rest.sh/bytes/64 --rsh-raw > sample.bin
```

Raw output bypasses Restish's structured body decoding and formatting. It still
uses the body exposed by the HTTP client after any content-encoding
decompression, so it is not a capture of the exact compressed wire bytes.
Use `-r` or `--rsh-raw` for raw response body bytes; `raw` is not an `-o`
format.

## Record Formats

Record formats can emit one item or event at a time:

- `ndjson`: one JSON value per line
- `lines`: one scalar value per line, without JSON string quotes
- plugin formats such as `csv` when implemented as record-oriented formatters

```bash
restish https://api.rest.sh/images -o ndjson -f body.self
restish https://api.rest.sh/images -f body.self -o lines
restish https://api.rest.sh/events --rsh-max-items 3 -o ndjson
```

The CSV formatter freezes its header from the first object batch. Later rows
that omit known fields get empty cells. Later rows that introduce new fields are
still emitted, but the new fields are ignored with a warning because CSV cannot
add columns after the header has already been written.

## Tables

```bash
restish https://api.rest.sh/images -o table --rsh-columns name,format,self
restish https://api.rest.sh/images -o table --rsh-sort-by name
```

Tables are for terminal inspection, not stable machine parsing.

## Filters And Scalar Lines

```bash
restish https://api.rest.sh/images -f body.name -o lines
```

Explicit scalar filters print without JSON string quotes. `-o lines` prints
arrays and streams of scalar values one value per line, and rejects structured
objects. Without a filter, `-r` writes the response body bytes and cannot be
combined with `-f`.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Defaults](../output-defaults/)
- [Filtering](/docs/guides/filtering/)
