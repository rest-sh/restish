---
title: Output Defaults
linkTitle: Output Defaults
weight: 31
description: Reference for default output choices on terminals, redirects, filters, pagination, and streams.
---

Restish chooses defaults to be useful interactively and predictable in scripts,
but explicit `-o` is always better when format matters.

## Main Rule

- TTY output defaults to `readable` for structured responses.
- Non-TTY structured output should be machine-friendly.
- `-o raw` preserves original body bytes.
- `-o json` and `-o yaml` produce complete documents.
- `-o ndjson` produces records.

## Examples

```bash
restish https://api.rest.sh/images
restish https://api.rest.sh/images -o json > images.json
restish https://api.rest.sh/images/jpeg -o raw > dragonfly.jpg
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

## Filtering

A filter changes what is rendered:

```bash
restish https://api.rest.sh/images -f body.self -r
```

Use `-r` for shell-friendly scalar output.

## Related Pages

- [Output](/docs/guides/output/)
- [Output Formats](../output-formats/)
- [Normalized Responses](/docs/concepts/normalized-responses/)
