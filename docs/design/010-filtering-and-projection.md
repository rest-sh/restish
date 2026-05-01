# Filtering And Projection

## Summary

Restish v2 applies filtering after response normalization and before
formatting. Users can query the normalized response with either shorthand path
syntax or jq syntax, and Restish can usually infer which language they meant.

Filtering is not just a rendering flourish. It is part of the data-flow model
that determines what value downstream output logic is working with.

## Goals

- expose the full normalized response, not just the body
- keep common cases lightweight
- preserve a powerful query language for complex transforms
- work predictably with pagination, streaming, and redirected output
- keep filtering semantics separate from formatting semantics

## Non-Goals

- inventing a third general-purpose query language
- filtering raw transport objects directly
- silently changing filter evaluation mode in ways users cannot reason about

## Filter Input Model

Filtering operates on a stable normalized document with these roots:

- `proto`
- `status`
- `headers`
- `links`
- `body`
- `@` for the full document

This means filters can reach:

- decoded body fields
- protocol metadata
- headers
- discovered hypermedia links

without forcing users to switch to a different API for each concern.

## Supported Languages

Restish supports two filter languages:

- shorthand path syntax for direct field access and lightweight projection
- jq for richer transformations, predicates, aggregation, and selection

This split is deliberate:

- shorthand keeps common access patterns fast to type
- jq keeps the ceiling high for complex workflows

## Language Detection

The default filter mode is `auto`.

In auto mode:

- if the expression begins with a recognized normalized-response root, treat it
  as shorthand
- otherwise treat it as jq
- if jq parsing fails and the expression is plausibly shorthand, fall back to
  shorthand instead of reporting a jq parse error

Typical shorthand expressions:

- `body.name`
- `body.items[0]`
- `headers.Content-Type`
- `links.next`
- `..url|[@ contains github]`

Typical jq expressions:

- `.body.items[] | select(.active)`
- `.body.items | length`
- `.body | map(.id)`

Explicit `--rsh-filter-lang` should always override auto-detection.

## Filter Execution Phase

Filtering happens after normalization and after the logical response shape is
known, but before formatter selection is finalized for the resulting value.

That means:

- normal bounded responses filter one normalized document
- paginated responses may filter either per-record or per-logical-collection
  depending on plan
- streams filter one event/item at a time unless the selected mode requires a
  bounded collection

This ties filtering directly into the output planner described in design 028.

## Per-Record Versus Whole-Collection Filters

Filters fall into two broad classes:

- **per-record filters** such as `body.id` or `.body.items[] | .name`
- **whole-collection filters** such as `.body | map(.id)`, `length`,
  `group_by(...)`, or `sort_by(...)`

The output planner must classify which kind of filter is being used because
that determines whether:

- the response can stream through incrementally
- pagination can emit records one by one
- the CLI must collect the full logical result first

If a filter cannot safely run incrementally, Restish should collect
automatically or fail clearly.

## Shorthand Semantics

Shorthand filtering is intended for path-style projection over the normalized
response document. It should stay simple and predictable:

- direct root selection
- explicit full-document selection via `@`
- nested object traversal
- array indexing
- lightweight projection helpers compatible with the shorthand library's model

Shorthand is not meant to become a partial jq clone.

## jq Semantics

jq remains the escape hatch for:

- selection and predicates
- reshaping
- aggregation
- sorting and grouping
- computed values

Restish should treat jq as an embedded query engine, not as something to
reinterpret in CLI-specific ways beyond the normalized input model and raw
output options.

## Planning Conflicts

Some options interact in ways users should not have to guess about.

Examples:

- `--rsh-headers` asks for header-oriented output
- `--rsh-filter` asks for a selected sub-value
- `--rsh-raw` asks for raw/plain output of the current selection

When options conflict, Restish should either:

- define a clear precedence and document it, or
- reject the combination with a clear error or warning

Silent discarding of user intent is not acceptable.

## `--rsh-raw`

`--rsh-raw` is the single raw/plain output control. Without a filter, it writes
the original response body bytes after transfer decoding. With a filter, it is a
presentation shortcut layered on top of filtering. It is meant for shell-friendly
display of filter results by:

- removing JSON quotes from scalar strings
- printing arrays of scalars one item per line

It should not try to invent a broad alternate formatting system, and `raw`
should not be reintroduced as an `-o` formatter name.

## Output Consequences

Once filtering selects a sub-value, Restish is no longer working with the
original raw response payload. That changes default output behavior for
non-TTY/stdout-redirected cases:

- if the result is a transformed value, default to JSON
- do not try to preserve raw bytes that no longer correspond to the selected
  result

This is a key interaction between filtering and design 009/028.

An explicit `-f @` is not the same as omitting a filter. No filter may use a
body-oriented display default, but `@` selects the full normalized response
document with `status`, `headers`, `links`, and `body`.

## Performance And Caching

jq compilation may be cached for efficiency, but that cache should be bounded
and scoped in a way that does not create unbounded memory growth for long-lived
embedders.

The cache is an implementation detail. The design requirement is:

- repeated use of the same jq expression should not require recompilation every
  time
- long-lived processes should not leak memory via unbounded expression caches

## Examples

Given this normalized response:

```json
{
  "proto": "HTTP/2",
  "status": 200,
  "headers": {
    "Content-Type": "application/json"
  },
  "links": {
    "next": "https://api.example.com/items?page=2"
  },
  "body": {
    "items": [
      {"id": 1, "name": "alpha", "active": true},
      {"id": 2, "name": "beta", "active": false}
    ]
  }
}
```

Common shorthand filters:

```bash
restish get https://api.example.com/items -f body.items[0].name
restish get https://api.example.com/items -f headers.Content-Type
restish get https://api.example.com/items -f links.next
```

Example jq filters:

```bash
restish get https://api.example.com/items -f '.body.items[] | select(.active) | .name'
restish get https://api.example.com/items -f '.body.items | length'
```

Example raw presentation of filtered values:

```bash
restish get https://api.example.com/items -f '.body.items[] | .name' --rsh-raw
```

which prints:

```text
alpha
beta
```

## Alternatives Considered

### Support Only jq

Too high-friction for common inspection tasks.

### Support Only Shorthand Paths

Too limiting for real transformation work.

### Filter Only The Body By Default

Would throw away a core advantage of Restish's normalized response model.

## Relationship To Other Designs

- Design 009 defines the normalized response document filters operate on.
- Design 011 and 012 affect whether filtering can run per page/event or needs
  collection semantics.
- Design 028 defines the planner that combines filter class with output family.
