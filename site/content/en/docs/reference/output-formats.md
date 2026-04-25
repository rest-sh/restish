---
title: Output Formats
linkTitle: Output Formats
weight: 30
description: Reference for built-in Restish output formats and formatting behavior.
---

Restish includes a small set of built-in output formats. They all render from
the same normalized response model, but they optimize for different jobs.

It helps to think about them in two families:

- **document formats** such as `json`, `yaml`, and `readable`
- **record formats** such as `ndjson` and stream-oriented plugins

That distinction matters because explicit format choice determines the framing
contract:

- document formats aim to produce one coherent result
- record formats emit one item or event at a time

## Default Selection

If you do not pass `-o`:

- a TTY defaults to `readable`
- a non-TTY target defaults to JSON for normalized structured output
- `image/*` responses on a TTY can switch to `image`

Explicit `-o <format>` always wins.

See [Output Defaults](../output-defaults/) for the TTY and redirect behavior in
more detail.

## Built-In Formats

### `readable`

Best for interactive terminal use.

- shows status and headers
- pretty-prints structured bodies
- uses color when the terminal supports it
- keeps the body copyable as valid JSON
- on paginated TTY output, keeps the same array/object framing while drawing it
  incrementally as pages arrive

```bash
restish https://api.rest.sh/images -o readable
```

### `raw`

Best when you need the original response bytes unchanged.

- ideal for redirects to files or pipes
- preserves binary payloads
- use `-o raw` or `-r` without a filter when the original wire payload matters
  more than decoded structured output

```bash
restish https://api.rest.sh/images/jpeg -o raw > dragonfly.jpg
```

### `json`

Encodes the decoded `body` value as formatted JSON.

- does not include status or headers
- useful after filtering or when you want a clean JSON document
- always emits one valid JSON document
- on paginated responses, automatically takes the document-oriented path

```bash
restish https://api.rest.sh/images -o json
```

### `ndjson`

Encodes one JSON value per line.

- best for paginated item-by-item processing
- best for live SSE / NDJSON stream consumption
- a good fit for shell loops and downstream tools like `jq`
- the explicit JSON format for record-by-record output

```bash
restish https://api.rest.sh/images -o ndjson -f 'body.self'
```

### `yaml`

Renders the decoded `body` as YAML.

```bash
restish https://api.rest.sh/images -o yaml
```

### `table`

Renders an array of objects as a table.

- use `--rsh-columns` to pick or order columns
- use `--rsh-sort-by` to sort rows before rendering
- best when the result is an array of similarly shaped objects

```bash
restish https://api.rest.sh/images -o table --rsh-columns name,format,self
```

### `gron`

Renders nested data as one assignment per line.

```bash
restish https://api.rest.sh/example -o gron
```

### `cbor`

Encodes the decoded `body` as CBOR bytes.

```bash
restish https://api.rest.sh/images -o cbor > images.cbor
```

### `image`

Renders `image/*` responses inline on a TTY.

- prefers terminal-native image display mechanisms when available
- falls back to a Unicode half-block renderer
- writes raw bytes unchanged when stdout is not a TTY

```bash
restish https://api.rest.sh/images/png -o image
```

## Filters Change What Gets Rendered

All output formats run after filtering. If a filter selects a sub-value,
Restish formats that value rather than the original full response.

```bash
restish https://api.rest.sh/example -f body.basics.profiles -o json
```

For shell-friendly scalar output, add `-r`:

```bash
restish https://api.rest.sh/example -f body.basics.profiles -r
```

For paginated or streaming record-by-record output, prefer `-o ndjson`:

```bash
restish https://api.rest.sh/images -o ndjson -f 'body.self'
```

## Pagination And Streaming Contracts

The practical rule is:

- `-o json` means one valid JSON document
- `-o yaml` means one valid YAML document
- `-o readable` means one coherent terminal-oriented view
- `-o ndjson` means one JSON value per line

That is why `-o json` collects paginated structured output into one document,
while `-o ndjson` is the explicit incremental format.

For live streams, prefer `ndjson` when you want machine-readable item-by-item
processing.

## Plugin Formats

Formatter plugins can add new names to `-o <name>`. Those plugins receive the
same normalized response document as built-in formatters.

Plugin formatters receive a short formatter session. For ordinary responses,
the full body usually arrives on the `start` message. For paginated or
event-stream output, the formatter can stay alive across many `item` messages
so formats like CSV can emit one header row followed by streamed records.

See also:

- [Output Guide](/docs/guides/output/)
- [Output Defaults](../output-defaults/)
- [Plugin Manifest Reference](../plugin-manifest/)
- [Plugin Message Reference](../plugin-messages/)
