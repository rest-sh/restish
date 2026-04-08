---
title: Output
linkTitle: Output
weight: 50
description: Understand how Restish normalizes responses and renders output formats.
---

# Output

Restish normalizes responses before rendering them through formatters.

That gives the CLI a stable internal response shape with:

- `proto`
- `status`
- `headers`
- `links`
- `body`

## Default Output Depends On Context

Restish uses adaptive defaults:

- on a TTY, the default is `readable`
- off a TTY, the default is `raw`
- for `image/*` content on a TTY, Restish can use the image formatter

That split is intentional. Interactive use needs context and formatting, while
pipes and scripts usually want the original response bytes.

## Common Output Formats

Select a formatter with `-o` or `--rsh-output-format`:

```bash
restish get https://httpbin.org/json -o readable
restish get https://httpbin.org/json -o json
restish get https://httpbin.org/json -o yaml
restish get https://httpbin.org/json -o raw
```

In practice:

- `readable` is best for terminal inspection
- `json` and `yaml` are good when you want the decoded response body
- `raw` preserves the original response body bytes
- `table` is useful for arrays of similar objects

## Readable Output

The readable formatter is designed for humans. It keeps useful HTTP context
visible and still renders the body in a copyable structured form.

That usually means:

- status and headers stay visible
- structured bodies are pretty-printed
- colors are used when stdout supports them

## Raw Output

Raw output is the best choice when you want to save or pipe the response body
unchanged:

```bash
restish get https://api.example.com/archive.tar.gz > archive.tar.gz
```

This is also why non-TTY output defaults to `raw`.

## Filtering Changes What Gets Rendered

When you filter a response, Restish renders the filtered value rather than the
original wire payload.

For example:

```bash
restish get https://httpbin.org/json -f body.slideshow.title
```

If the filter selects a sub-value, that result is rendered in the chosen output
format. In non-interactive mode, filtered sub-values are emitted as structured
data rather than pretending they are still the original raw bytes.

## Raw Filter Output

Use `-r` or `--rsh-raw` with filters when you want shell-friendly scalar
results:

```bash
restish get https://api.example.com/items -f '.body.items[] | .name' -r
```

That strips quotes from strings and prints arrays one item per line.

## Table Output

For arrays of objects, `-o table` can be easier to scan:

```bash
restish get https://api.example.com/items -o table --rsh-columns id,name,status
restish get https://api.example.com/items -o table --rsh-sort-by name
```

Use `--rsh-columns` to pick visible fields and `--rsh-sort-by` to control row
ordering.

## Silent Output

Use `-S` or `--rsh-silent` when you only care about the exit code:

```bash
restish -S get https://api.example.com/health
```

This is useful in shell checks and CI probes.

## Related Guides

- [Filtering](../filtering/)
- [Streaming](../streaming/)
- [Output Formats](../reference/output-formats/)

Source material:

- [`docs/design/009-response-normalization-and-output.md`](/Users/daniel/src/restish2/docs/design/009-response-normalization-and-output.md)
- [`docs/design/025-image-rendering.md`](/Users/daniel/src/restish2/docs/design/025-image-rendering.md)
