# Document And Record Output

## Summary

Restish needs to support two different output experiences:

- a single logical response document
- an incremental stream of records

Those two experiences overlap in implementation, but they should not be
confused in the user interface. In particular, explicit document formats such
as `json` and `yaml` must always produce valid documents, while line-oriented
record processing should be explicit.

This design introduces a user-facing distinction between **document output**
and **record output**. Pagination, streaming, filtering, auto rendering,
`--rsh-print`, and formatter plugins should all compose through that
distinction.

## Problem

Restish currently has several competing goals:

- automatic pagination should "just work" for list endpoints
- SSE and NDJSON responses should emit output immediately
- TTY users should get readable, colored, low-latency feedback
- shell users should be able to process one item at a time
- `-o json` must always produce valid JSON
- filtered and paginated output should stay consistent with unpaginated output
- formatter plugins should be able to render paginated and streaming output
  without the host buffering everything unnecessarily

For stream-shaped responses, "immediately" means once the server or intermediary
has delivered a complete event or record to the client. Restish should not add
its own full-response buffering, but it cannot make an upstream-buffered
response appear incremental.

The old pagination split was too coarse:

- non-collect mode streamed one item at a time
- collect mode buffered everything and re-entered the normal formatter path

That was simple, but it caused two kinds of surprises:

1. **Grammar surprises.** An explicit format like `-o json` could become an
   invalid stream of adjacent JSON values instead of one JSON document.
2. **Shape surprises.** A paginated list endpoint could render as a wrapped
   object, an array, or a stream of bare items depending on whether a `next`
   link was present in that particular response.

The second problem matters because pagination is discovered dynamically. At
request time Restish may not know whether an endpoint will paginate; it learns
that only after inspecting the first response's hypermedia links or per-API
pagination config.

The user experience therefore needs a stable rule that does not let the same
endpoint silently change output semantics just because one invocation had a
second page and another did not.

## Design

### Two Output Families

Restish should treat output formats as belonging to one of two families.

#### Document Formats

Document formats represent one logical result:

- `auto`
- `json`
- `yaml`
- future document-oriented plugins

These formats must preserve their framing guarantees:

- `-o json` always emits one valid JSON document
- `-o yaml` always emits one valid YAML document
- `-o auto` emits the default body/value presentation

Document formats may need to buffer paginated data or otherwise switch into a
collect-like execution path to uphold that contract.

#### Record Formats

Record formats represent one item or event at a time:

- `ndjson`
- `lines` for scalar values
- `csv`
- future record-oriented plugins

These formats optimize for first-byte latency and shell-friendliness. They are
allowed to emit one result at a time because that is their explicit contract.

### Explicit Format Wins

An explicit `-o <format>` must determine the framing contract up front.

That means:

- `-o json` implies document semantics
- `-o yaml` implies document semantics
- `-o auto` implies document semantics for bounded values and human
  presentation for TTY streams
- `-o ndjson` implies record semantics
- `-o lines` implies scalar value-per-line semantics
- plugin formats should declare whether they are document-oriented,
  record-oriented, or both

Pagination may change *how* Restish executes, but not *what contract* the user
asked for.

### Redirected Output Saves Body Bytes By Default

When stdout is not a TTY and the user did not request a filter, metadata
shortcut, pagination collection, or output format, Restish should write the
response body bytes after HTTP content-encoding decompression. Redirection is
therefore a byte-preserving save path for JSON, CBOR, YAML, images, text, and
unknown payloads.

This keeps a clean product distinction:

- redirection saves the response body bytes
- `-o <format>` transforms the decoded body or selected value
- filters and metadata shortcuts render normalized values
- `--rsh-collect` opts into a synthesized multi-page document

The rule is:

- when Restish is still writing the original unmodified payload, raw body-byte
  output is meaningful for redirected stdout
- when an explicit filter selects a scalar, Restish may print the scalar plainly
  because the selected value is already shell-native
- when Restish is writing a normalized, collected, or transformed structured
  value without an explicit format, redirected output should default to pretty
  JSON; explicit `--rsh-print=b` selects compact rendered output for scripts

Practically, saving a CBOR response keeps CBOR bytes:

```bash
restish api.rest.sh/formats/cbor > response.cbor
```

Converting that same response to JSON is explicit:

```bash
restish api.rest.sh/formats/cbor -o json > response.json
```

### `-o json` Preserves Document Framing

`-o json` should always emit one valid JSON document, but it should not change
what a filter means. Pagination mode determines the data model; output format
determines the serialization contract.

This keeps the JSON contract simple:

- paginated bounded responses can still become one valid JSON document
- filtered per-item results can be collected into a JSON array
- collection transforms remain explicit through `--rsh-collect`

It also means `-o json` is never a streaming wire format. Users who want JSON
values incrementally should choose `-o ndjson`.

### `ndjson` Is The Explicit Record JSON Format

Restish should add a built-in `ndjson` formatter for record-oriented machine
consumption.

NDJSON is a good fit because:

- each output item is still valid JSON
- one item per line works naturally with shell pipelines
- it matches the existing incremental stream mental model
- it has an established de facto specification and media type

`-o ndjson` is the correct answer for:

- paginated item-by-item processing
- SSE / NDJSON stream processing
- shell loops, `jq -r`, `while read`, and `xargs`

### `lines` Is The Explicit Scalar Line Format

Restish should add a built-in `lines` formatter for shell-native scalar values.
It prints strings without JSON quotes and prints arrays or streams of scalars
one value per line.

`-o lines` is the correct answer when a filter selects values for shell tools
such as `while read`, `xargs`, or command substitution. It must reject objects
and arrays containing objects so line output does not silently erase structured
shape. Users who need structured data should choose `-o json`, `-o ndjson`, or
another structured format.

### Auto Output Has Two Internal Modes

Auto output is a human/default body/value format, but it should still provide
low-latency feedback. HTTP status and header context is not formatter output;
the CLI print layer writes it when `--rsh-print` includes `h` or `H`.

For bounded non-paginated responses, auto output writes only the selected value
to stdout:

- text as text
- images through terminal image presentation when supported
- structured values as pretty JSON on a terminal
- binary bodies as a short notice on a terminal

When the selected value is response metadata or the full response envelope,
auto and table-like renderers should display that metadata instead of
assuming every useful field lives under `body`. Status, headers, links, and
body are all legitimate output roots.

For pagination and true streams, auto output may switch to a record-oriented
presentation mode internally while preserving a coherent human interface:

- write the response status and headers once when `--rsh-print` includes `h`
- render each item or event as its own pretty-printed JSON block
- separate records clearly
- keep ANSI highlighting when color is enabled

This gives TTY users feedback as quickly as possible without pretending the
result is one JSON document.

### Filtering Must Be Planned

Filters fall into two broad categories:

- **per-record filters** such as `body.id` or `body.{format,self}`
- **whole-collection filters** such as `length`, `.body | map(.id)`,
  `sort_by(...)`, or `group_by(...)`

Per-record filters can compose with record formats and paginated incremental
execution. Each record is evaluated through a mini normalized response where the
current item or stream event is under `body`; this keeps `body.*` filters
consistent with full-response filtering.

Whole-collection filters require a full logical collection, even when the
response arrived in pages. Restish should not try to infer this from the filter
syntax because shorthand and jq both contain expressions that are hard to
classify safely and predictably. Users ask for whole-collection semantics with
`--rsh-collect`.

Restish therefore needs an output planning step that classifies:

- whether the underlying result is bounded or unbounded
- whether the selected format is document or record oriented
- whether `--rsh-collect` requests whole-collection evaluation
- whether pagination needs to preserve a wrapper object via `items_path`

Without `--rsh-collect`, paginated filters run per item against that mini
response wrapper. Document formats then render the filtered item results as one
valid document; record formats can emit them as records.

### Pagination Must Preserve Shape

For paginated bounded responses, Restish should preserve the logical response
shape whenever possible:

- a bare paginated array stays an array in document formats
- a wrapped object with `items_path: data` should stay an object whose `data`
  field becomes the merged collection
- paginated filtering should operate on the logical merged shape only when
  `--rsh-collect` is set

This avoids the surprise where the same endpoint produces an object sometimes
and a stream of bare items other times depending only on whether a `next` link
was present.

### True Streams Remain Record-Oriented

Open-ended SSE and NDJSON responses differ from paginated collections in one
important way: there may never be a natural end-of-document.

For those responses:

- record formats are the natural fit
- auto output should render records incrementally in interactive terminals
- document formats like `json` should be treated as bounded-document requests

In practice, that means `-o json` on an unbounded stream should either:

- buffer until EOF and only succeed for bounded streams, or
- fail with a clear error telling the user to use `-o ndjson`

The latter is the better UX for clearly live feeds.

### Plugin Support

Formatter plugins should declare whether they support:

- full document rendering
- record rendering
- both

The existing formatter session protocol already supports streamed `item`
messages, which makes it a natural fit for record-oriented plugins such as CSV.

Document-oriented plugins should continue to receive a full logical body for
ordinary responses and collected paginated document output.

The host should keep owning:

- pagination
- filtering
- normalized response planning
- SSE / NDJSON parsing

Plugins should stay renderers, not alternate execution pipelines.

## Planner Algorithm

The output planner should make these decisions in order:

1. classify the underlying response as:
   - bounded non-paginated
   - bounded paginated
   - unbounded stream
2. resolve the selected format family:
   - explicit document format
   - explicit record format
   - adaptive default based on TTY and whether output is still raw
3. decide filter scope:
   - none
   - per-record pagination filter
   - whole-collection filter requested by `--rsh-collect`
4. decide whether the logical result must preserve a wrapper object via
   pagination config such as `items_path`
5. choose execution strategy:
   - direct bounded document render
   - collected paginated document render
   - incremental record render
   - auto incremental human mode
   - fail because the requested format cannot be satisfied safely

This planning step should happen before bytes are written to stdout. Once output
starts, Restish should not silently discover it chose the wrong framing mode.

## Execution Strategies

### Bounded Document Render

Use when:

- the response is bounded
- no pagination collection merge is needed
- the selected format is document-oriented

### Collected Paginated Document Render

Use when:

- the response is paginated and bounded
- `--rsh-collect` requested whole-collection semantics, or
- no per-record filter transform is active and the selected output contract is
  a document format

### Incremental Record Render

Use when:

- the selected format is record-oriented
- the filter can run per record
- the response is either paginated or streamed

### Auto Incremental Human Mode

Use when:

- the selected format is `auto`
- low latency is valuable
- the result is paginated or streamed

This is still a document-oriented human contract, but it is implemented with a
record-oriented internal execution strategy.

## Failure Cases

The planner should fail clearly when the user asks for a contract Restish
cannot satisfy safely.

Examples:

- `-o json` on a clearly unbounded live stream
- a whole-collection jq transform combined with forced incremental-only output
- a formatter plugin that supports only record rendering when the selected mode
  requires a full document

Clear failure is better than silently emitting invalid or misleading output.

## Behavior Matrix

### Paginated Bounded Response

- default TTY: auto, low-latency human output with response status and headers
  on stdout
- default redirected non-TTY without filters or formats: first response body
  bytes, with automatic pagination skipped
- `-o json`: valid pretty JSON document by default; unfiltered pagination
  merges items, filtered pagination renders filtered item results
- `-o yaml`: valid YAML document; unfiltered pagination merges items, filtered
  pagination renders filtered item results
- `-o auto`: render progressively for humans
- `-o ndjson`: stream one item per line
- `-o lines`: stream scalar values one per line, error on structured values
- `-o csv`: stream one record per row if the formatter supports it

### True SSE / NDJSON Stream

- default TTY: auto incremental event output
- default non-TTY: NDJSON-style item output when the payload is structured
- `-o ndjson`: one JSON value per line
- `-o auto`: incremental human-readable event view
- `-o json`: not a streaming mode; should require a bounded stream or error

### Filtering

- per-record filter + record format: run incrementally
- per-record filter + document format: filter each item, then render those
  values as a valid document
- whole-collection filter + paginated response: use `--rsh-collect`
- `--rsh-collect`: force whole-collection evaluation

## Examples

Human-in-the-loop TTY scan of a paginated endpoint:

```bash
restish api.rest.sh/images -f 'body.{format,self}'
```

This should render incrementally in auto mode, one record at a time, so
the user sees feedback immediately.

Shell pipeline processing paginated items:

```bash
restish api.rest.sh/images -f 'body.id' -o lines | while read id; do
  echo "process $id"
done
```

Dump all IDs to a JSON file:

```bash
restish api.rest.sh/images --rsh-collect -f '.body | map(.id)' -o json > ids.json
```

Because the filter is whole-collection, the command opts into collection
explicitly. `-o json` makes the output serialization explicit for scripts.

Export paginated data to CSV:

```bash
restish api.rest.sh/images -o csv -f 'body.{id,format,self}'
```

Follow a live event stream with less noise:

```bash
restish api.example.com/events -f 'body.{type,id,message}'
```

This should print events as they arrive in auto mode.

## Alternatives Considered

### Keep document and record output implicit

This preserves a smaller implementation surface, but it leads directly to the
bugs and surprises seen in paginated JSON output. Users need stable guarantees
from explicit formats.

### Stream `json` by writing fragments

This makes latency look good, but it violates the plain meaning of `json`.
Adjacent JSON objects are not a valid JSON document.

### Stream `json` by writing a live array wrapper

This works only for bounded record streams and still needs special handling for
wrapper objects, collection-wide filters, and truly unbounded event streams.
It is not a good universal contract for `-o json`.

### Depend on `encoding/json/v2` for the design

Go's experimental `encoding/json/v2` and `encoding/json/jsontext` packages are
promising for lower-level JSON writing, but they remain experimental and gated
behind `GOEXPERIMENT=jsonv2`. They may be useful as an internal implementation
detail, but they should not define the user-facing output model.

## Notes

This design intentionally separates **framing semantics** from **execution
strategy** and filter scope:

- framing semantics come from the selected output mode
- filter scope comes from pagination mode: item-by-item by default,
  whole-collection with `--rsh-collect`
- execution strategy is chosen by Restish to satisfy that contract without
  changing filter meaning

That distinction keeps pagination, filtering, auto rendering, and plugins
composable without requiring the user to reason about internal control flow.
