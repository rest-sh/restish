# `restish-csv` Plugin

## Summary

`restish-csv` is a concrete formatter-hook plugin that renders array-shaped JSON
response bodies as CSV.

It exists primarily as the canonical formatter-plugin example: small enough to
read in one sitting, but real enough to demonstrate useful output shaping and a
few non-trivial decisions around rows, columns, schema freezing, and value
encoding.

## Goals

- provide a useful non-core formatter example
- validate the formatter session protocol for bounded, paginated, and streamed
  output
- keep CSV rendering rules deterministic and easy to reason about
- avoid forcing the host to buffer everything just to satisfy one plugin

## Non-Goals

- becoming a spreadsheet modeling tool
- inventing a sophisticated flattening scheme for arbitrary nested objects
- supporting every scalar or mixed-array input shape

## Why CSV Is A Good Example

CSV is a strong example because it is:

- clearly presentation-oriented
- broadly useful
- stateful enough to validate the formatter session model
- small enough to stay learnable

It sits at exactly the boundary where formatter plugins should shine: the host
owns request execution and normalization, while the plugin owns final bytes.

## Manifest

The plugin advertises:

- `name: csv`
- `hooks: ["formatter"]`
- `formatter_names: ["csv"]`

This makes `-o csv` available through the generic formatter selection path.

## Session Model

The plugin participates in the formatter session protocol.

For an ordinary non-streaming response, Restish sends a `start` message whose
`response.body` holds the full value. For paginated and event-stream output,
Restish sends `item` messages with one value at a time.

The plugin therefore has two modes:

- one-shot document-like mode for bounded input
- incremental mode for paginated or streamed records

## Accepted Input Shapes

Whenever `restish-csv` receives a value, it expects it to be:

- one object, or
- an array of objects

The plugin treats these as errors:

- a value that is neither an object nor an array of objects
- any array item that is not an object

That narrow scope is intentional. The plugin is meant to be a focused tabular
formatter, not a universal coercion layer.

## Column Discovery

For bounded input, the plugin:

1. scans every row object
2. builds the union of all object keys
3. sorts the column names for deterministic output
4. writes one CSV header row
5. writes one CSV record per body item

Sorted columns trade first-row ordering for deterministic output. That makes the
formatter easier to test and easier to compare across runs.

## Cell Encoding

Cell encoding is intentionally simple:

- `null` becomes an empty field
- strings are emitted as raw text values
- numbers and booleans are rendered as their scalar text form
- arrays and objects are JSON-encoded into a single cell

This keeps the plugin predictable without inventing a custom flattening scheme
for nested data.

## Streaming And Pagination Behavior

For paginated and event-stream output:

1. Restish starts one plugin process.
2. The plugin waits for `formatter` messages on stdin.
3. The first object(s) it receives determine the CSV header.
4. The plugin writes one header row, then one data row per streamed object.

The streaming path is intentionally stricter than the one-shot path:

- it accepts either one object or an array of objects per `item` message
- once the header is written, later objects may not introduce new fields

That tradeoff keeps the formatter genuinely stream-friendly. CSV requires a
header before later rows can be emitted, so a plugin that wants true streaming
must either freeze the schema early or buffer indefinitely. `restish-csv`
chooses the former and errors on schema drift.

## Schema Freeze Rule

The schema-freeze rule is one of the defining behaviors of this plugin.

Before header emission:

- discover columns from the first available object set

After header emission:

- accept only rows whose keys are a subset of the frozen header
- write missing fields as empty cells
- reject newly introduced columns

This is the key design choice that lets the plugin stay incremental.

## Error Model

The plugin should fail clearly for:

- unsupported input shape
- mixed arrays containing non-object items
- schema drift after header freeze
- malformed formatter messages

Once bytes have already been written, these errors are still real failures. The
host may already have partial output, but that is acceptable for a stream-aware
formatter as long as the failure is explicit.

## Why It Matters Architecturally

`restish-csv` demonstrates the intended boundary for formatter hooks:

- Restish still owns HTTP, decoding, filtering, pagination, and normalization
- the plugin receives a normalized response model rather than `*http.Response`
- the plugin owns final bytes on stdout
- the plugin can hold just enough state to stay consistent across paginated and
  event-stream output without forcing the host to buffer everything

This is exactly the kind of output transformation that should be easy to move
out of process without changing the core CLI pipeline.

One notable non-goal is sharing built-in syntax highlighting. Formatter plugins
receive a `color` hint, but they are responsible for any ANSI or styling they
want to emit. The core readable formatter's highlighting helpers remain an
in-process implementation detail.

## Alternatives Considered

### Keep CSV Built Into The Main Binary

Possible, but less valuable as a plugin example.

### Use A More Minimal Example Like `gron`

Too simple to really validate the session protocol.

### Flatten Nested Objects Into Multiple Columns

Friendlier for some spreadsheet workflows, but it adds a lot of policy to what
should remain a focused reference implementation.

## Relationship To Other Designs

- Design 019 defines the formatter-hook session model.
- Design 028 defines the document-vs-record planner that decides when this
  plugin sees full bodies versus incremental items.
