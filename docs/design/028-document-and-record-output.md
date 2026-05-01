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
and **record output**. Pagination, streaming, filtering, readable rendering,
and formatter plugins should all compose through that distinction.

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

- `json`
- `yaml`
- `readable`
- future document-oriented plugins

These formats must preserve their framing guarantees:

- `-o json` always emits one valid JSON document
- `-o yaml` always emits one valid YAML document
- `-o readable` emits one coherent human-readable response view

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
- `-o readable` implies document semantics
- `-o ndjson` implies record semantics
- `-o lines` implies scalar value-per-line semantics
- plugin formats should declare whether they are document-oriented,
  record-oriented, or both

Pagination may change *how* Restish executes, but not *what contract* the user
asked for.

### Redirected Output Defaults To JSON Documents

When stdout is not a TTY and Restish is writing normalized structured output,
the default should be JSON instead of raw bytes.

This is a deliberate shift from the earlier v2 behavior. The previous non-TTY
default of `raw` was good for byte-preserving passthrough, but it interacts
poorly with pagination, filtering, readable fallbacks, and other operations
that produce normalized values rather than the original wire payload.

The revised rule is:

- when Restish is still writing the original unmodified raw payload, raw output
  remains meaningful and available explicitly through `-r`
- when an explicit filter selects a scalar, Restish may print the scalar plainly
  because the selected value is already shell-native
- when Restish is writing a normalized or transformed value, redirected output
  should default to JSON

Practically, this means users can write:

```bash
restish api.rest.sh/images -f '.body | map(.id)' > ids.json
```

without needing to add `-o json`.

### `-o json` Implies Collection Semantics

`-o json` should automatically enable `--rsh-collect`-style execution for
paginated responses.

This keeps the JSON contract simple:

- paginated bounded responses become one valid JSON document
- filtered collection transforms stay coherent
- wrapper objects such as `body.items` can be preserved

It also means `-o json` is never a streaming mode. Users who want JSON values
incrementally should choose `-o ndjson`.

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

### Readable Output Has Two Internal Modes

Readable output is always a document-oriented human format, but it should still
provide low-latency feedback.

For bounded non-paginated responses, readable output stays as it is today:

- HTTP preamble
- blank line
- pretty JSON body

When the selected value is response metadata or the full response envelope,
readable and table-like renderers should display that metadata instead of
assuming every useful field lives under `body`. Status, headers, links, and
body are all legitimate output roots.

For pagination and true streams, readable output should switch to a
record-oriented presentation mode internally while preserving a coherent human
interface:

- write the HTTP preamble once
- then render each item or event as its own pretty-printed JSON block
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
execution.

Whole-collection filters require a full logical collection, even when the
response arrived in pages.

Restish therefore needs an output planning step that classifies:

- whether the underlying result is bounded or unbounded
- whether the selected format is document or record oriented
- whether the filter is per-record or whole-collection
- whether pagination needs to preserve a wrapper object via `items_path`

If a filter cannot be safely streamed, Restish should collect automatically or
surface a clear diagnostic.

### Pagination Must Preserve Shape

For paginated bounded responses, Restish should preserve the logical response
shape whenever possible:

- a bare paginated array stays an array in document formats
- a wrapped object with `items_path: data` should stay an object whose `data`
  field becomes the merged collection
- paginated filtering should operate on the logical merged shape in document
  mode, not on a single page at a time

This avoids the surprise where the same endpoint produces an object sometimes
and a stream of bare items other times depending only on whether a `next` link
was present.

### True Streams Remain Record-Oriented

Open-ended SSE and NDJSON responses differ from paginated collections in one
important way: there may never be a natural end-of-document.

For those responses:

- record formats are the natural fit
- readable output should render records incrementally
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
3. classify the filter as:
   - none
   - per-record
   - whole-collection
4. decide whether the logical result must preserve a wrapper object via
   pagination config such as `items_path`
5. choose execution strategy:
   - direct bounded document render
   - collected paginated document render
   - incremental record render
   - readable incremental human mode
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
- the selected format is document-oriented, or
- the filter requires whole-collection semantics

### Incremental Record Render

Use when:

- the selected format is record-oriented
- the filter can run per record
- the response is either paginated or streamed

### Readable Incremental Human Mode

Use when:

- the selected format is `readable`
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

- default TTY: readable, low-latency human output
- default non-TTY: JSON document output
- `-o json`: collect/merge, valid JSON document
- `-o yaml`: collect/merge, valid YAML document
- `-o readable`: render progressively for humans
- `-o ndjson`: stream one item per line
- `-o lines`: stream scalar values one per line, error on structured values
- `-o csv`: stream one record per row if the formatter supports it

### True SSE / NDJSON Stream

- default TTY: readable incremental event output
- default non-TTY: NDJSON-style item output when the payload is structured
- `-o ndjson`: one JSON value per line
- `-o readable`: incremental human-readable event view
- `-o json`: not a streaming mode; should require a bounded stream or error

### Filtering

- per-record filter + record format: run incrementally
- per-record filter + document format: apply over the logical collection shape
- whole-collection filter + paginated response: collect automatically
- `--rsh-collect`: force whole-collection evaluation

## Examples

Human-in-the-loop TTY scan of a paginated endpoint:

```bash
restish api.rest.sh/images -f 'body.{format,self}'
```

This should render incrementally in readable mode, one record at a time, so
the user sees feedback immediately.

Shell pipeline processing paginated items:

```bash
restish api.rest.sh/images -f 'body.id' -o lines | while read id; do
  echo "process $id"
done
```

Dump all IDs to a JSON file:

```bash
restish api.rest.sh/images -f '.body | map(.id)' > ids.json
```

Because stdout is redirected, Restish should default to document JSON output.
Because the filter is whole-collection, pagination should collect automatically.

Export paginated data to CSV:

```bash
restish api.rest.sh/images -o csv -f 'body.{id,format,self}'
```

Follow a live event stream with less noise:

```bash
restish api.example.com/events -f 'body.{type,id,message}'
```

This should print events as they arrive in readable mode.

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
strategy**:

- framing semantics come from the selected output mode
- execution strategy is chosen by Restish to satisfy that contract

That distinction keeps pagination, filtering, readable rendering, and plugins
composable without requiring the user to reason about internal control flow.
