---
title: Output Formats
linkTitle: Output Formats
weight: 30
description: Reference for Restish output formats and document-versus-record behavior.
---

Output formats decide how a normalized response is rendered after decoding,
pagination or streaming, and filtering.

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

For file redirects, image responses already write body bytes:

```bash
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
restish https://api.rest.sh/bytes/64 --rsh-raw > sample.bin
```

Raw output bypasses Restish's structured body decoding and formatting. It still
uses the body exposed by the HTTP client after any content-encoding
decompression, so it is not a capture of the exact compressed wire bytes.
Use `-r` or `--rsh-raw` for raw output; `raw` is not an `-o` format.

## Record Formats

Record formats can emit one item or event at a time:

- `ndjson`: one JSON value per line
- plugin formats such as `csv` when implemented as record-oriented formatters

```bash
restish https://api.rest.sh/images -o ndjson -f body.self
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

## Tables

```bash
restish https://api.rest.sh/images -o table --rsh-columns name,format,self
restish https://api.rest.sh/images -o table --rsh-sort-by name
```

Tables are for terminal inspection, not stable machine parsing.

## Filters And Raw Scalars

```bash
restish https://api.rest.sh/images -f '.body[] | .name' -r
```

Raw mode strips quotes from scalar strings and prints array items one per line.
Without a filter, `-r` writes the response body bytes.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Defaults](../output-defaults/)
- [Filtering](/docs/guides/filtering/)
