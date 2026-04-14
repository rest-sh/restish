---
title: Output Formats
linkTitle: Output Formats
weight: 30
description: Reference for built-in Restish output formats and formatting behavior.
---

Restish includes a small set of built-in output formats. They all render from
the same normalized response model, but they optimize for different jobs.

## Default Selection

If you do not pass `-o`:

- a TTY defaults to `readable`
- a non-TTY target defaults to `raw`
- `image/*` responses on a TTY can switch to `image`

Explicit `-o <format>` always wins.

## Built-In Formats

### `readable`

Best for interactive terminal use.

- shows status and headers
- pretty-prints structured bodies
- uses color when the terminal supports it
- keeps the body copyable as valid JSON

```bash
restish https://api.rest.sh/images -o readable
```

### `raw`

Best when you need the original response bytes unchanged.

- ideal for redirects to files or pipes
- preserves binary payloads
- default for non-TTY stdout

```bash
restish https://api.rest.sh/images/jpeg -o raw > dragonfly.jpg
```

### `json`

Encodes the decoded `body` value as formatted JSON.

- does not include status or headers
- useful after filtering or when you want a clean JSON document

```bash
restish https://api.rest.sh/images -o json
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

## Plugin Formats

Formatter plugins can add new names to `-o <name>`. Those plugins receive the
same normalized response document as built-in formatters.

See also:

- [Output Guide](/docs/guides/output/)
- [Plugin Manifest Reference](../plugin-manifest/)
- [Plugin Message Reference](../plugin-messages/)
