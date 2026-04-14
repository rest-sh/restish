# `restish-csv` Plugin

## Summary

`restish-csv` is a concrete formatter-hook plugin that renders array-shaped JSON
response bodies as CSV.

It exists primarily as the canonical formatter-plugin example: small enough to
read in one sitting, but real enough to demonstrate useful output shaping and a
few non-trivial decisions around rows, columns, and value encoding.

## Problem

The generic formatter-hook design in
[`019-hook-plugins.md`](/Users/daniel/src/restish2/docs/design/019-hook-plugins.md)
explains the transport, but it helps to also have one concrete formatter plugin
in the repository that answers practical questions:

- what should a formatter manifest look like
- what request payload does Restish send to a formatter plugin
- how much output logic should live in the plugin versus the core CLI
- what kinds of formatting behavior are a good fit for plugins

CSV is a strong example because it is clearly presentation-oriented, broadly
useful, and not entangled with Restish's built-in streaming or readable-output
paths.

## Design

The plugin advertises:

- `name: csv`
- `hooks: ["formatter"]`
- `formatter_names: ["csv"]`

The plugin participates in the formatter session protocol. For an ordinary
non-streaming response, Restish sends a `start` message whose `response.body`
holds the full value. For paginated and event-stream output, Restish sends
`item` messages with one value at a time.

Whenever `restish-csv` receives a value, it expects it to be an object or an
array of objects. It then:

1. scans every row object
2. builds the union of all object keys
3. sorts the column names for deterministic output
4. writes one CSV header row
5. writes one CSV record per body item

Cell encoding is intentionally simple:

- `null` becomes an empty field
- strings are emitted as their raw text values
- numbers, booleans, arrays, and objects are JSON-encoded into a single cell

That keeps the plugin predictable without inventing a custom flattening scheme
for nested data.

### Session Behavior

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

## Scope

The example is intentionally narrow and intentionally learnable.

The plugin currently treats these inputs as errors:

- a value that is neither an object nor an array of objects
- any array item that is not an object
- in streaming mode, any later row that introduces fields not present when the
  header was established

That keeps the example focused on one happy path instead of trying to guess how
scalar responses or mixed arrays should map onto tabular output.

## Why It Matters Architecturally

`restish-csv` demonstrates the intended boundary for formatter hooks:

- Restish still owns HTTP, decoding, filtering, pagination, and normalization
- the plugin receives a normalized response model rather than `*http.Response`
- the plugin owns final bytes on stdout
- the plugin can now hold just enough state to stay consistent across paginated
  and event-stream output without forcing the host to buffer everything

This is exactly the kind of output transformation that should be easy to move
out of process without changing the core CLI pipeline.

One notable non-goal is sharing built-in syntax highlighting. Formatter plugins
currently receive a `color` hint, but they are responsible for any ANSI or
styling they want to emit. The core readable formatter's highlighting helpers
remain an in-process implementation detail for now.

## Alternatives Considered

### Keep CSV built into the main binary

Possible, but it is more valuable as an example of a useful-but-optional output
mode that does not need to live in the core formatter registry.

### Use a more minimal example like `gron`

`gron` is simpler, but almost too simple as a teaching example. CSV forces the
plugin to make a few realistic decisions about column discovery and nested
value handling without becoming large or protocol-heavy.

### Flatten nested objects into multiple columns

That would produce friendlier spreadsheets for some APIs, but it would add a
lot of policy to what is supposed to be a small reference implementation. JSON
encoding nested values into a single field keeps the example easier to reason
about.

## Notes

The implementation lives in
[`cmd/restish-csv/main.go`](/Users/daniel/src/restish2/cmd/restish-csv/main.go),
with focused behavior tests in
[`cmd/restish-csv/main_test.go`](/Users/daniel/src/restish2/cmd/restish-csv/main_test.go)
and CLI integration coverage in
[`internal/cli/plugin_hook_test.go`](/Users/daniel/src/restish2/internal/cli/plugin_hook_test.go).
