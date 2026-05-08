---
title: Output Defaults
linkTitle: Output Defaults
weight: 31
description: Reference for default output choices on terminals, redirects, filters, pagination, and streams.
---

Restish chooses defaults to be useful interactively and predictable in scripts,
but an explicit format is better when a script depends on a specific shape.

The defaults are designed around where stdout is going. A terminal should show
something readable to a human. A pipe or file should get the response body
bytes unless you ask Restish to filter, collect, or format the response.

## Main Rule

- TTY output defaults to `readable` for structured responses.
- Redirected unfiltered output writes response body bytes.
- `--rsh-raw` writes response body bytes explicitly and works the same way for
  JSON, CBOR, images, text, and other response types.
- `-o json` and `-o yaml` produce complete documents.
- `-o ndjson` produces records.
- `-o lines` produces one scalar value per line.
- Output format does not change paginated filter scope; use `--rsh-collect`
  for whole-collection filters.

## Examples

```bash
restish https://api.rest.sh/images
restish https://api.rest.sh/images -o json > images.json
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
restish https://api.rest.sh/content/cbor > response.cbor
restish https://api.rest.sh/content/cbor -o json > response.json
restish https://api.rest.sh/events --rsh-max-items 3 -o ndjson
```

## Filtering

A filter changes what is rendered:

```bash
restish https://api.rest.sh/images -f body.self -o lines
```

Explicit scalar filters print without JSON string quotes. Use `-o lines` for
shell-friendly arrays or streams of scalar values. Once you filter, you are no
longer asking for response body bytes; you are asking Restish to render the
selected value.

Raw response output is based on the body after HTTP content-encoding
decompression, not the exact compressed wire transfer. `raw` is not an `-o`
output format, and raw mode cannot be combined with filters.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Normalized Responses](/docs/concepts/normalized-responses/)
