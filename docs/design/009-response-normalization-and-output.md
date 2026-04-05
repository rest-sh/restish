# Response Normalization And Output

## Summary

Restish v2 turns each HTTP response into a normalized internal `Response`
structure before filtering or formatting. That creates a stable seam between
HTTP transport concerns and presentation concerns.

From there, output behavior is chosen based on format selection and whether
stdout is a TTY.

## Problem

Restish needs to work well in two very different modes:

- interactive terminal use, where users want context, readability, and color
- scripting or piping, where users want predictable machine-friendly output and
  lossless handling of raw bytes

If those concerns are mixed too early, the result becomes hard to extend and
hard to reason about. The same response should be usable by filters, formatters,
stream handlers, and other higher-level behaviors without each part reaching
back into `*http.Response`.

## Design

The core design is to normalize responses once, then treat formatting as a
separate concern.

The normalized response includes:

- protocol version
- numeric status
- flattened headers
- discovered hypermedia links
- decoded body value
- original raw body bytes

That shape is important because different output modes need different views of
the same response. For example:

- human-readable output wants status, headers, and a pretty body
- JSON output usually wants just the decoded body value
- raw output wants the original bytes without re-encoding
- filtering wants a stable document with `proto`, `status`, `headers`, `links`,
  and `body`

The formatting model is intentionally adaptive:

- explicit `-o <format>` wins
- TTY + `image/*` content type → `image` formatter (inline terminal rendering)
- TTY default is `readable`
- non-TTY default is `raw`

There is one important exception: when a non-TTY invocation filters down to a
sub-value rather than the full response, Restish emits JSON for that filtered
value instead of raw bytes, because the filtered result is no longer the
original wire payload.

The readable formatter is designed to preserve useful HTTP context while keeping
the body copyable as valid JSON. Non-interactive modes prioritize faithful data
transfer and scriptability over presentation.

## Example

An HTTP response like:

```http
HTTP/2 200 OK
Content-Type: application/json
Link: <https://api.example.com/items?page=2>; rel="next"

{"items":[{"id":1,"name":"alpha"}]}
```

is normalized into a structure shaped like:

```json
{
  "proto": "HTTP/2",
  "status": 200,
  "headers": {
    "Content-Type": "application/json",
    "Link": "<https://api.example.com/items?page=2>; rel=\"next\""
  },
  "links": {
    "next": "https://api.example.com/items?page=2"
  },
  "body": {
    "items": [
      {
        "id": 1,
        "name": "alpha"
      }
    ]
  }
}
```

From that same normalized response:

- `-o readable` shows status, headers, and a pretty body
- `-o json` emits just the decoded `body`
- default non-TTY output emits the original raw bytes

## Alternatives Considered

### Format directly from `*http.Response`

This would couple every formatter to transport-layer details and make filtering
and middleware behavior harder to compose cleanly.

### Always output only the response body

That is convenient for scripts, but it throws away useful protocol context for
interactive use. Restish needs both modes to feel natural.

### Use one default format everywhere

A single default sounds simpler, but terminal usage and pipe usage optimize for
different things. Adaptive defaults are worth the small amount of extra logic.

## Notes

The current implementation reflects this design directly:

- `internal/output/response.go` defines the normalized `Response` type and
  normalization pipeline
- `internal/output/format.go` defines formatter selection and default behavior
- `internal/output/readable_formatter.go` renders the interactive view
- `internal/output/json_formatter.go` and `internal/output/raw_formatter.go`
  cover the common machine-oriented paths
- `internal/output/image_formatter.go` renders `image/*` responses inline on TTY
- `internal/cli/http.go` applies filtering before selecting the formatter; it also
  performs content-type-aware dispatch for `image/*` before calling `Select()`

One design detail worth preserving is that the normalized response carries both
decoded `Body` and original `Raw` bytes. That dual representation is what lets
Restish support both friendly reformatted output and exact byte-preserving raw
output without forcing a choice too early in the pipeline.
