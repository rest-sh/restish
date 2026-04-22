# Response Normalization And Output

## Summary

Restish v2 turns each bounded HTTP response into a normalized internal
`Response` structure before filtering or formatting. That creates a stable seam
between HTTP transport concerns and presentation concerns.

From there, output behavior is chosen based on format selection, content type,
and whether stdout is a TTY.

## Goals

- provide one stable response model for filters, formatters, and workflow
  commands
- preserve both decoded structure and original payload bytes where needed
- support both human-oriented and machine-oriented output modes
- keep formatting decisions separate from HTTP transport details
- avoid corrupting text or binary payloads during normalization

## Non-Goals

- exposing `*http.Response` directly to every downstream consumer
- forcing all output modes to operate on the same exact body representation
- losing wire fidelity once a structured decode was possible

## Normalization Boundary

Normalization happens after:

- status line and headers are available
- response encodings such as gzip/brotli are decoded
- the response body is fully read for bounded responses
- the content registry has had a chance to decode the body

Normalization does **not** apply to the same extent for true streams; streaming
uses the separate path defined in design 012.

## Normalized Response Schema

Conceptually, the normalized response includes:

- `proto`: protocol version string
- `status`: numeric status code
- `headers`: flattened header map
- `links`: normalized hypermedia relation map
- `body`: decoded or otherwise normalized logical body value
- `raw`: original response bytes after transfer decoding but before logical
  re-encoding

That schema is the stable document filters and most formatters consume.

## Header Model

Headers are normalized into a presentation-friendly map keyed by header name.
The exact casing policy should be stable and documented by implementation, but
the design requirement is that:

- header values remain inspectable and scriptable
- downstream filters do not need raw `http.Header` semantics

If a richer multi-value header representation is ever required, it should be
added deliberately rather than by leaking transport-specific data structures.

## Body Representation Classes

The normalized model should preserve the distinction between:

- structured decoded values
- printable text
- raw binary payloads

That distinction is essential for correct output defaults and for preserving
fidelity.

### Structured Values

JSON, YAML, CBOR, msgpack, and similar responses become structured values that
filters and formatters can traverse.

### Printable Text

Text payloads should remain text when that preserves meaning. Human-oriented
formatters should not unnecessarily re-wrap plain text bodies as JSON strings if
that would make the output less faithful or less readable.

### Raw Binary

Unknown or binary content must remain bytes. Coercing unknown binary into a Go
`string` is a design bug because it corrupts the payload and produces misleading
later output.

## Why `raw` Also Exists

The normalized response carries both decoded `body` and original `raw` bytes.
That dual representation is what lets Restish support:

- friendly reformatted output
- exact byte-preserving output
- filtering over structured data
- image/content-type-aware dispatch decisions

without forcing an irreversible choice too early in the pipeline.

## Hypermedia Integration

Normalization also includes hypermedia link extraction. The normalized `links`
map is the same conceptual layer used by:

- pagination
- the `links` command
- filters that project specific relations

This keeps link handling out of individual formatter or command implementations.

## Output Selection Rules

The formatting model is intentionally adaptive:

- explicit `-o <format>` wins
- TTY + `image/*` content type may dispatch to the image formatter
- TTY default is `readable`
- non-TTY defaults distinguish between original raw bytes and normalized data

Explicit conflicting modes should be surfaced clearly. If one option asks for
headers-only output and another asks for filtered body projection, the user
should not have to guess which one silently won.

## Output Families

Restish separates output formats into two families:

- **document formats** such as `json`, `yaml`, and `readable`
- **record formats** such as `ndjson` and record-oriented formatter plugins

Document formats must preserve framing guarantees:

- `-o json` always emits one valid JSON document
- `-o yaml` always emits one valid YAML document
- `-o readable` emits one coherent human-readable response view

Design 028 defines the higher-level planner that combines normalization results
with pagination, streaming, and filtering.

## Default Output Behavior

When stdout is not a TTY and Restish is writing normalized structured data, the
default output is JSON rather than raw bytes. Explicit `-o raw` remains the
escape hatch for original wire payloads.

The practical rule is:

- if Restish is still outputting the original payload unchanged, raw output is
  meaningful
- if Restish is outputting a transformed or selected logical value, default to
  JSON

## Readable Output Contract

The readable formatter is designed to preserve useful HTTP context while keeping
the body copyable and understandable.

A bounded readable response typically includes:

- status line / protocol
- selected headers
- a visually separated body view

Readable output is primarily for humans, but it should still preserve the
meaning of text and structured content rather than aggressively prettifying
everything into a less faithful shape.

## Text And Binary Handling

Output behavior must not corrupt data:

- printable text bodies should render as text in human-oriented formats
- unknown binary should remain bytes, not coerced strings
- JSON formatters should emit stable JSON without unnecessary HTML escaping

Binary-to-string coercion is a design bug because it damages fidelity and later
formatting behavior.

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

- `-o readable` shows status, headers, and a human-oriented body
- `-o json` emits just the decoded `body`
- `-o ndjson` emits one JSON value per line when the logical result is
  record-shaped

## Alternatives Considered

### Format Directly From `*http.Response`

Too tightly coupled to transport details.

### Always Output Only The Response Body

Too weak for interactive use and protocol inspection.

### Use One Default Format Everywhere

Too simplistic for the TTY/non-TTY split Restish needs.

## Relationship To Other Designs

- Design 003 defines how content types are decoded before normalization.
- Design 010 defines filtering over the normalized response document.
- Design 011 and 012 define how pagination and streams diverge from the bounded
  response path.
- Design 025 defines content-type-aware image dispatch.
- Design 028 defines document-versus-record output planning.
