---
title: Output Defaults
linkTitle: Output Defaults
weight: 31
description: Reference for default output choices on terminals, redirects, filters, pagination, and streams.
---

Restish chooses defaults to be useful interactively and predictable in scripts,
but an explicit format is better when a script depends on a specific shape.

The defaults are designed around where stdout is going. A terminal should show
something readable to a human. A pipe or file should usually get a stable
machine-readable value. Binary image downloads are the exception: when stdout
is redirected, Restish writes the image body bytes so the saved file opens
normally.

## Main Rule

- TTY output defaults to `readable` for structured responses.
- Non-TTY structured output defaults to JSON.
- Redirected image responses are written as decoded body bytes.
- `--rsh-raw` preserves decoded body bytes for generic byte streams.
- `-o json` and `-o yaml` produce complete documents.
- `-o ndjson` produces records.

## Examples

```bash
restish https://api.rest.sh/images
restish https://api.rest.sh/images > images.json
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
restish https://api.rest.sh/bytes/64 --rsh-raw > sample.bin
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

## Filtering

A filter changes what is rendered:

```bash
restish https://api.rest.sh/images -f body.self -r
```

Use `-r` for shell-friendly scalar output. Once you filter, you are no longer
asking for the response body bytes; you are asking Restish to render the selected
value. Raw response output is based on the body after HTTP content-encoding
decompression, not the exact compressed wire transfer. The
[Output guide](/docs/guides/output/) covers that processing model.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Normalized Responses](/docs/concepts/normalized-responses/)
