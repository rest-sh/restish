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

The implementation may cap text sniffing for otherwise unknown bytes, but that
cap must not apply once the response is already known to be text. Known text
includes `text/*` media types, Markdown media types or file-like URLs, and
other content types or URL extensions that map to a real syntax highlighter
such as CSS, XML, or JavaScript. Large text assets should still render as text
in auto output; size alone is not a reason to fall back to JSON string
encoding.

### Raw Binary

Unknown or binary content must remain bytes. Coercing unknown binary into a Go
`string` is a design bug because it corrupts the payload and produces misleading
later output.

Auto TTY output should not dump arbitrary binary bytes and should not render
byte slices as base64 JSON strings. If Restish has no content-aware presentation
for the body, the auto formatter should show a short body notice with the byte
count, content type when available, and the raw-output escape hatch.
Content-aware binary presentations such as terminal images remain allowed when
stdout supports them.

## Why `raw` Also Exists

The normalized response carries both decoded `body` and original `raw` bytes.
That dual representation is what lets Restish support:

- friendly reformatted output
- exact byte-preserving output
- filtering over structured data
- image/content-type-aware dispatch decisions

without forcing an irreversible choice too early in the pipeline.

For redirected unfiltered responses, raw output is byte-oriented. It must write
the original response body bytes after transfer decoding, not a Go value
formatted through `fmt` and not a decode/re-encode approximation. Once the user
selects a transformed logical value with a filter, byte fidelity no longer
applies to that transformed value. Shell-friendly filtered scalar output belongs
to the `lines` output format instead.

## Hypermedia Integration

Normalization also includes hypermedia link extraction. The normalized `links`
map is the same conceptual layer used by:

- pagination
- the `links` command
- filters that project specific relations

This keeps link handling out of individual formatter or command implementations.

## Output Selection Rules

The formatting model is intentionally adaptive:

- `--rsh-print` selects the HTTP exchange parts Restish writes to stdout
- `-o <format>` formats the rendered body/value selected by `--rsh-print=b`
- `-f` selects the body/value used by `b`
- omitted `-f` selects `body`
- `-f @` selects the full normalized response envelope
- `auto` is the default output format
- TTY + `image/*` content type may dispatch to the image formatter
- interactive unfiltered requests default to `--rsh-print=hbpc`: response
  status/headers plus a pretty, colored body on stdout
- redirected non-TTY output preserves original body bytes when no filter,
  metadata shortcut, pagination collection, or explicit output format is set;
  this raw-download path bypasses response middleware so installed plugins
  cannot silently rewrite saved files
- non-TTY filtered or transformed values default to the rendered body part with
  pretty formatting (`bp`); explicit `--rsh-print=b` keeps JSON output compact
  for scripts

Built-in and plugin `-o` formatters render only the selected body/value. They
must not write HTTP status lines, request headers, or response headers. The CLI
print layer owns those exchange parts so `--rsh-print=h`, `--rsh-print=H`, and
`--rsh-print=b` compose consistently across all output formats.

Explicit print strings should be literal. If Restish cannot satisfy a requested
print part, it should fail with a targeted error instead of silently dropping
data.

## Output Families

Restish separates `-o` output formats into two families:

- **document formats** such as `auto`, `json`, `yaml`, `cbor`, `table`, and
  `gron`
- **record/value formats** such as `ndjson`, `lines`, and record-oriented
  formatter plugins

`raw` is not an `-o` output format and is not a public print part. Raw byte
output is the automatic non-TTY path for unfiltered, unformatted responses.
Plain scalar line output is requested with `-o lines`.

Document formats must preserve framing guarantees:

- `-o json` always emits one valid JSON document
- `-o yaml` always emits one valid YAML document
- `-o cbor` always emits one CBOR document for the normalized body
- `-o auto` emits the default human/body presentation for the selected value
- `-o lines` emits one scalar value per line and rejects structured values

Built-in formatter contracts:

| Format | Contract |
| --- | --- |
| `auto` | Default selected-value presentation: text as text, images in capable TTYs, binary notices on TTYs, and structured values as highlighted JSON on TTYs or JSON off TTYs. |
| `json` | Body or selected value as stable JSON, without HTML escaping. |
| `yaml` | Body or selected value as YAML. |
| `cbor` | Body as CBOR for binary-safe structured pipelines. |
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

The practical rule is:

- if Restish is still outputting the original payload unchanged, write body bytes
- if stdout is an interactive terminal and no explicit filter is set, print the
  response status line, headers, and formatted body to stdout
- if Restish is outputting an explicitly filtered scalar, print the scalar
  plainly without JSON string quotes
- if Restish is outputting a transformed or selected structured value, preserve
  its shape with the selected/default formatter, defaulting to pretty JSON for
  structured non-TTY values unless `--rsh-print=b` is explicit

## Auto Output Contract

The `auto` formatter is designed to keep the body copyable and understandable.
The `--rsh-print=auto` selector decides whether that formatter appears alone
or inside an HTTP transcript.

Auto output is primarily for humans, but it should still preserve the
meaning of text and structured content rather than aggressively prettifying
everything into a less faithful shape.

Auto text bodies use presentation helpers only when they do not change the
underlying response model:

- `text/markdown`, `text/x-markdown`, and Markdown-looking response URLs may be
  rendered with a terminal Markdown renderer in color-capable TTYs
- other highlightable text content types or URL extensions may use Chroma
  syntax highlighting
- `text/plain`, `application/octet-stream`, unknown text, non-TTY output, or
  renderer failures fall back to text bytes when they are safe to display

Auto output should choose the text presentation path before the structured
JSON presentation path for recognized text responses. For example, a large
`text/css` or `application/javascript` response should be displayed as CSS or
JavaScript text and highlighted when color is enabled, not marshalled as one
large JSON string. The small-body safety cap used for unknown byte sniffing is
only a binary-safety heuristic; it is not a display limit for declared text.

Auto output should choose a binary notice before the structured JSON
presentation path for byte-backed binary responses. A binary body is not a JSON
string merely because Go's JSON encoder can serialize `[]byte` as base64.

The Markdown renderer derives its style from the active Restish theme so
Markdown bodies and generated help feel coherent with auto JSON and HTTP
context. `GLAMOUR_STYLE` remains an explicit operator escape hatch for users
who want Glamour's environment-driven behavior instead of Restish's theme. This
is presentation only; filtered output, machine formats, and redirected bytes
must not depend on terminal Markdown rendering.

## Theme Configuration

Auto output and printed HTTP transcripts use a Chroma style for HTTP preambles,
JSON-like structured values, special scalar values, and Restish's
bracket-depth token types. The
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

The `restish config theme set <source>` command reads a theme JSON or JSONC
document from a local path or fetches it from a URL, validates it, and saves its
entries into the top-level config `theme` object while preserving JSONC comments
where possible. It also records the resolved source in `theme_source` so repeated
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

which first tries `themes/dark.json`, then falls back to the repository's root
`dark.json` when the `themes/` path is not found.

Official bundled theme names can be discovered with:

```bash
restish config theme list
```

and installed by name:

```bash
restish config theme set one-dark-pro
```

Name-only theme installation reads the matching `themes/<name>.json` file
embedded in the Restish binary. This keeps the list and installer deterministic
for each release. The saved `theme_source` uses `official:<name>` for bundled
themes. Users who want mutable or third-party themes can still install by local
path, URL, or GitHub shorthand.

The `restish config theme reset` command removes both `theme` and
`theme_source` from config, preserving unrelated JSONC comments where possible,
and restores the built-in theme for the current process. `unset` is accepted as
an alias for users looking for the inverse of `set`.

## Text And Binary Handling

Output behavior must not corrupt data:

- printable text bodies should render as text in human-oriented formats
- declared or recognized text bodies should render as text regardless of body
  size, provided the bytes are safe to display
- unknown binary should remain bytes, not coerced strings
- auto TTY output should summarize unsupported binary bodies instead of
  writing raw bytes or base64 JSON strings
- redirected or piped unfiltered responses should write the payload bytes
  exactly unless the user explicitly selected a different formatter
- JSON formatters should emit stable JSON without unnecessary HTML escaping

Binary-to-string coercion is a design bug because it damages fidelity and later
formatting behavior.

Terminal image rendering is a TTY presentation feature. It must not run when
stdout is redirected to a file, because doing so corrupts downloads such as
PNG, JPEG, or SVG assets.

## Decision: `--rsh-print`

V2 uses `--rsh-print` to control which HTTP exchange parts are written to
stdout. This follows the shape of HTTPie's print selector while keeping
Restish's existing `-o` flag focused on body/value formatting.

Supported print parts are:

- `H`: request line and request headers
- `B`: request body
- `h`: response status line and response headers
- `b`: rendered response body or filtered value
- `p`: pretty formatting for rendered request/response bodies
- `c`: color

`--rsh-print=auto` chooses the common case:

- TTY + no explicit filter: `hbpc`
- redirected/piped stdout + no explicit filter or output transform: body bytes,
  bypassing response middleware
- explicit filters, metadata shortcuts, collection, or explicit `-o` formats:
  `bp`, with color enabled when terminal or environment rules allow it

The important separation is:

- `--rsh-print` decides which parts go to stdout
- `-o` decides how `b` is rendered
- `-f` decides which value `b` renders

Explicit print strings are honored literally where possible. For example,
`--rsh-print=b` renders only the body slot even on a terminal, while
`--rsh-print=hb` renders a response transcript. Because `b` lacks `p`, it also
acts as the compact rendered-output mode for scripts. This makes `auto`
opinionated but keeps the escape hatch small and predictable.

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

- default interactive output writes response status, headers, and body to stdout
- `-o json` emits the selected value, normally the decoded `body`
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
