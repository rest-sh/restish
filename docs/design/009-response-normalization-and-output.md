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
- common dotted filters such as `headers.Date` work case-insensitively
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

For an unfiltered response, `-r` raw output is byte-oriented. It must write the
original response body bytes after transfer decoding, not a Go value formatted
through `fmt` and not a decode/re-encode approximation. Once the user selects a
transformed logical value with a filter, byte fidelity no longer applies to that
transformed value; `-r` is therefore rejected with filters. Shell-friendly
filtered scalar output belongs to the `lines` output format instead.

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
- redirected non-TTY output preserves original body bytes when no filter,
  metadata shortcut, pagination collection, or explicit output format is set
- non-TTY filtered or transformed values use formatter defaults, with JSON as
  the machine-readable default for structured values

Explicit conflicting modes should be surfaced clearly. If one option asks for
headers-only output and another asks for filtered body projection, the user
should not have to guess which one silently won.

## Output Families

Restish separates `-o` output formats into two families:

- **document formats** such as `json`, `yaml`, `cbor`, `table`, `gron`, and
  `readable`
- **record/value formats** such as `ndjson`, `lines`, and record-oriented
  formatter plugins

`raw` is not an `-o` output format. Raw byte output is requested with `-r` and
applies only to the original response body. Plain scalar line output is
requested with `-o lines`.

Document formats must preserve framing guarantees:

- `-o json` always emits one valid JSON document
- `-o yaml` always emits one valid YAML document
- `-o cbor` always emits one CBOR document for the normalized body
- `-o readable` emits one coherent human-readable response view
- `-o lines` emits one scalar value per line and rejects structured values

Built-in formatter contracts:

| Format | Contract |
| --- | --- |
| `json` | Body or selected value as stable JSON, without HTML escaping. |
| `yaml` | Body or selected value as YAML; full-response TTY output may include a short status preamble. |
| `cbor` | Body as CBOR for binary-safe structured pipelines. |
| `readable` | HTTP preamble plus body presentation for humans. |
| `table` | Object or array-of-object bodies as a fixed-width table; non-tabular values fall back to JSON. |
| `gron` | Deterministic `json.<path> = <value>;` assignments for grep-friendly inspection. |
| `ndjson` | One compact JSON record per line for item streams and event streams. |
| `lines` | Scalars, or arrays of scalars, as unquoted lines; objects and arrays of objects are errors. |
| `image` | Terminal presentation for image responses; redirected output keeps original bytes. |

`--rsh-columns` and `--rsh-sort-by` only affect `table`. Table column discovery
uses stable key ordering, later rows can add extra columns, and long cells are
truncated for terminal readability. That makes `table` a human scanning format,
not a lossless interchange format.

Design 028 defines the higher-level planner that combines normalization results
with pagination, streaming, and filtering.

## Default Output Behavior

When stdout is not a TTY and the user has not asked Restish to filter, collect,
or reformat the response, the default output is the original response body bytes
after HTTP content-encoding decompression. This keeps shell redirection aligned
with the common file-saving mental model, even for structured binary formats
such as CBOR.

Explicit `-r` requests the same original-body byte output regardless of whether
stdout is a TTY. It remains incompatible with filters and output formats because
those options ask Restish to render a normalized value rather than preserve the
response body.

The practical rule is:

- if Restish is still outputting the original payload unchanged, write body bytes
- if Restish is outputting an explicitly filtered scalar, print the scalar
  plainly without JSON string quotes
- if Restish is outputting a transformed or selected structured value, preserve
  its shape with the selected/default formatter, defaulting to JSON for
  structured non-TTY values

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

Readable text bodies use presentation helpers only when they do not change the
underlying response model:

- `text/markdown`, `text/x-markdown`, and Markdown-looking response URLs may be
  rendered with a terminal Markdown renderer in color-capable TTYs
- other highlightable text content types or URL extensions may use Chroma
  syntax highlighting
- `text/plain`, `application/octet-stream`, unknown text, non-TTY output, or
  renderer failures fall back to the original printable bytes

The Markdown renderer derives its style from the active Restish theme so
Markdown bodies and generated help feel coherent with readable JSON and HTTP
preambles. `GLAMOUR_STYLE` remains an explicit operator escape hatch for users
who want Glamour's environment-driven behavior instead of Restish's theme. This
is presentation only; filtered output, machine formats, and redirected bytes
must not depend on terminal Markdown rendering.

## Readable Theme Configuration

Readable output uses a Chroma style for HTTP preambles, JSON-like structured
values, special scalar values, and Restish's bracket-depth token types. The
built-in theme is the default, but users may override individual token styles
with a top-level `theme` object in config:

```json
{
  "theme_source": "https://example.com/theme.json",
  "theme": {
    "key": "#5fafd7",
    "header_key": "#87afd7",
    "string": "#afd787",
    "status_2xx": "bold #afd787"
  }
}
```

Theme values are Chroma style descriptors. Keys may be Chroma token names such
as `NameTag` or Restish aliases such as `key`, `keyword`, `function`, `class`,
`builtin`, `operator`, `url`, `date`, `bracket_0`, `header_key`, `status_2xx`,
`status_3xx`, and `status_error`. If `header_key` is omitted, HTTP response
header names inherit the `key` style for compatibility. User entries overlay
the built-in theme rather than
replacing it wholesale, so small config snippets can change one or two colors
without redefining every token.

Invalid theme keys or invalid style descriptors are config errors. Restish
should fail early during startup instead of silently producing partially styled
output.

The `restish config theme set <source>` command reads a theme JSON document
from a local path or fetches it from a URL, validates it, and saves its entries
into the top-level config `theme` object while preserving JSONC comments where
possible. It also records the resolved source in `theme_source` so repeated
remote installs of the same source can run without another trust prompt. A
first install of a new remote source prints the resolved URL and asks for
confirmation before fetching unless `--yes` is set. Theme files and downloads
are capped at 256 KiB. The JSON is a direct token map:

```json
{
  "key": "#ffffff",
  "status_2xx": "bold #00ff00"
}
```

For convenience, `<source>` may be a local path, an `http` or `https` URL, or a
GitHub `user/repo` shorthand. Local paths are stored as absolute paths. The
GitHub shorthand resolves to the repository's root `theme.json` through
GitHub's raw content host. A GitHub shorthand may also include an optional
theme name:

```bash
restish config theme set user/repo dark
```

which resolves to the repository's root `dark.json`.

The `restish config theme reset` command removes both `theme` and
`theme_source` from config, preserving unrelated JSONC comments where possible,
and restores the built-in theme for the current process. `unset` is accepted as
an alias for users looking for the inverse of `set`.

## Text And Binary Handling

Output behavior must not corrupt data:

- printable text bodies should render as text in human-oriented formats
- unknown binary should remain bytes, not coerced strings
- redirected or piped unfiltered responses should write the payload bytes
  exactly unless the user explicitly selected a different formatter
- JSON formatters should emit stable JSON without unnecessary HTML escaping

Binary-to-string coercion is a design bug because it damages fidelity and later
formatting behavior.

Terminal image rendering is a TTY presentation feature. It must not run when
stdout is redirected to a file, because doing so corrupts downloads such as
PNG, JPEG, or SVG assets.

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
