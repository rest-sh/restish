---
title: Output
linkTitle: Output
weight: 50
description: Understand how Restish normalizes responses and renders output formats.
---

Restish normalizes responses before rendering them through formatters.

That gives the CLI a stable internal response shape with:

- `proto`
- `status`
- `headers`
- `links`
- `body`

That normalized model is the reason filtering, table output, pagination, and
plugin formatters all work together instead of feeling like unrelated features.

## The Practical Rule

Think about output in two modes:

- document output
- record output

In practice that usually maps to:

- interactive inspection with `readable`
- machine-friendly documents with `json` or `yaml`
- machine-friendly record streams with `ndjson`

Restish adapts its defaults to match those two jobs.

## Default Output Depends On Context

Restish uses adaptive defaults:

- on a TTY, the default is `readable`
- off a TTY, normalized structured output defaults to JSON
- for `image/*` content on a TTY, Restish can use the image formatter

That split is intentional. Interactive use needs context and formatting, while
redirects and scripts usually want a stable structured result.

That means you often do not need to remember a flag for the common case.

## Common Output Formats

Select a formatter with `-o` or `--rsh-output-format`:

```bash
restish https://api.rest.sh/images -o readable
restish https://api.rest.sh/images -o json
restish https://api.rest.sh/images -o ndjson
restish https://api.rest.sh/images -o yaml
restish https://api.rest.sh/images -o raw
```

Those produce noticeably different shapes from the same response:

```text
HTTP/2.0 200 OK
Content-Type: application/cbor
Link: </schemas/ImageItemList.json>; rel="describedby"

[
  {
    format: "jpeg"
    name: "Dragonfly macro"
    self: "/images/jpeg"
  }
  ...
]
```

```json
[
  {
    "format": "jpeg",
    "name": "Dragonfly macro",
    "self": "/images/jpeg"
  }
]
```

```yaml
- format: jpeg
  name: Dragonfly macro
  self: /images/jpeg
```

In practice:

- `readable` is best for terminal inspection
- `json` and `yaml` are good when you want the decoded response body as one
  complete document
- `ndjson` is good when you want one record at a time
- `raw` preserves the original response body bytes
- `table` is useful for arrays of similar objects

If you are unsure, start with the default. Add `-o` only once you know what job
the output needs to do next.

## Format Selection Cheat Sheet

- `readable`: best interactive default on a terminal
- `json`: one machine-friendly document
- `yaml`: one human-readable document format
- `ndjson`: one JSON value per line
- `raw`: exact response bytes
- `table`: arrays of similar objects
- `image`: inline image display on supported terminals

## Readable Output

The readable formatter is designed for humans. It keeps useful HTTP context
visible and still renders the body in a copyable structured form.

That usually means:

- status and headers stay visible
- structured bodies are pretty-printed
- colors are used when stdout supports them

This is the format you want when you are exploring a response and trying to
understand what the API returned.

For paginated TTY output, readable mode now keeps the same array or wrapper
shape as non-paginated output while drawing the body incrementally as pages
arrive.

## Raw Output

Raw output is the best choice when you want to save or pipe the response body
unchanged:

```bash
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
```

Use this for:

- file downloads
- binary payloads
- exact redirects into another command
- any case where reformatting would be a bug

When you want a decoded structured document instead, just redirect normally:

```bash
restish https://api.rest.sh/images > images.json
```

## Filtering Changes What Gets Rendered

When you filter a response, Restish renders the filtered value rather than the
original wire payload.

For example:

```bash
restish https://api.rest.sh/example -f body.basics.profiles
```

Example output:

```json
[
  {
    "network": "Github",
    "url": "https://github.com/danielgtaylor"
  },
  {
    "network": "Dev Blog",
    "url": "https://dev.to/danielgtaylor"
  },
  {
    "network": "LinkedIn",
    "url": "https://www.linkedin.com/in/danielgtaylor"
  }
]
```

If the filter selects a sub-value, that result is rendered in the chosen output
format. In non-interactive mode, filtered sub-values are emitted as structured
data rather than pretending they are still the original raw bytes.

That distinction matters: once you filter, you are no longer asking for the
wire payload. You are asking for a transformed value.

## Raw Filter Output

Use `-r` or `--rsh-raw` with filters when you want shell-friendly scalar
results:

```bash
restish https://api.rest.sh/images -f '.body[] | .name' -r
```

That strips quotes from strings and prints arrays one item per line.

This is the fastest path to shell-friendly output.

Example output:

```text
Dragonfly macro
Origami under blacklight
Andy Warhol mural in Miami
Station in Prague
Chihuly glass in boats
```

## Table Output

For arrays of objects, `-o table` can be easier to scan:

```bash
restish https://api.rest.sh/images -o table --rsh-columns name,format,self
restish https://api.rest.sh/images -o table --rsh-sort-by name
```

Example output:

```text
╔════════════════════════════╤════════╤══════════════╗
║ name                       │ format │ self         ║
╟────────────────────────────┼────────┼──────────────╢
║ Dragonfly macro            │ jpeg   │ /images/jpeg ║
║ Origami under blacklight   │ webp   │ /images/webp ║
║ Andy Warhol mural in Miami │ gif    │ /images/gif  ║
║ Station in Prague          │ png    │ /images/png  ║
║ Chihuly glass in boats     │ heic   │ /images/heic ║
╚════════════════════════════╧════════╧══════════════╝
```

Use `--rsh-columns` to pick visible fields and `--rsh-sort-by` to control row
ordering.

Use table output when the response is list-shaped and you care about scanning
records quickly rather than preserving nested structure.

## Record Output With NDJSON

Use `-o ndjson` when you want one JSON value per line:

```bash
restish https://api.rest.sh/images -o ndjson -f 'body.self'
```

This is the right format for:

- paginated item-by-item shell processing
- true SSE / NDJSON response streams
- piping into tools that expect one record at a time

It also avoids overloading `-o json`, which always means one complete JSON
document.

## Silent Output

Use `-S` or `--rsh-silent` when you only care about the exit code:

```bash
restish -S https://api.rest.sh/
```

This is useful in shell checks and CI probes.

## A Useful Output Progression

Most users end up using these formats in roughly this order:

1. default `readable` while learning an API
2. `-o ndjson` or `-f` plus `-r` for record-by-record extraction
3. `-o table` for list endpoints
4. `raw` for downloads and exact redirects

Once you know that progression, output decisions get simpler.

## Related Guides

- [Filtering](../filtering/)
- [Streaming](../streaming/)
- [Images in the Terminal](../images-in-the-terminal/)
- [Output Formats](../reference/output-formats/)

Source material:

- [Design Records](/docs/contributing/design-records/)
