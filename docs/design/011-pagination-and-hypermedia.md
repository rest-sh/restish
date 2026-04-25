# Pagination And Hypermedia

## Summary

Restish v2 treats link discovery and pagination as part of the normal response
pipeline. Hypermedia parsers extract normalized links from headers or response
bodies, and pagination builds on those links to follow multi-page collections
automatically.

This is a cross-cutting subsystem rather than a single command feature. It
affects:

- normal GET requests
- the `links` command
- output planning for bounded collections
- filters that operate over paginated data

## Goals

- make common paginated APIs work automatically
- normalize link discovery across several hypermedia styles
- preserve one logical response shape for document-oriented output
- allow incremental record output for record-oriented formats
- avoid accidental infinite loops or silent shape changes

## Non-Goals

- hard-coding one API-specific pagination style into the core
- assuming pagination is only exposed via HTTP `Link` headers
- letting pagination rewrite explicit output contracts

## Two Layers

The design has two explicit layers:

1. hypermedia parsing
2. pagination planning and execution

Keeping those layers separate matters because link extraction is useful on its
own, even when no multi-page traversal happens.

## Hypermedia Parsing

Hypermedia parsing normalizes discovered links into a simple `rel -> uri` map.
Built-in parsers currently recognize:

- HTTP `Link` headers
- HAL `_links`
- JSON-LD or TSJ `@id`
- JSON:API top-level `links`
- Siren `links`

All discovered links are resolved to absolute URLs before being stored. This
lets downstream behavior treat links uniformly regardless of how they were
represented on the wire.

The HTTP `Link` header parser always runs for bounded responses. Body-based
hypermedia parsers may be lazy, because parsing structured bodies solely to
populate `links` is wasted work when the selected output never reads links.
Lazy parsing must still produce the same normalized link map when `links` is
accessed by pagination, the `links` command, or a filter.

## Parser Precedence And Merge Rules

Several parsers may discover links from the same response. The design should
therefore define stable merge behavior.

The preferred rule is:

- header and body parsers all contribute to one normalized relation map
- if multiple parsers discover the same relation with the same target, keep it
- if multiple parsers disagree on the same relation target, prefer the more
  explicit parser according to a stable precedence order and surface ambiguity
  in diagnostics where helpful

The important point is determinism. The resulting relation map should not depend
on parser registration accident or map iteration order.

## Relation Model

The normalized relation model is intentionally simple:

- relation name -> absolute URI

This sacrifices some fidelity from richer hypermedia formats, but it is the
right abstraction for:

- pagination
- `links` command output
- lightweight link inspection

If Restish ever needs a richer link object model, it should be added as a new
design layer rather than smuggled into the current `rel -> uri` contract.

## Pagination Triggers

Pagination execution uses the normalized link map plus optional per-API config.
For GET requests, if a `next` link is present, Restish can continue fetching
pages automatically.

Per-API pagination config can refine how page data is interpreted:

- `items_path` extracts the collection from a nested body field
- `next_path` can extract a next-page URL from the body

This lets Restish handle both standard hypermedia and APIs whose collection
wrappers need one extra hint.

Filters that select response metadata rather than body records normally disable
automatic pagination. For example, `-f headers`, `-f headers.Date`, and
`-f status` ask about the first response envelope, not about every body page.
Fetching additional pages for those filters produces repeated or misleading
metadata and violates the user's selected data model.

## Pagination Safety

Pagination is bounded by safety rules:

- context cancellation must be checked between pages
- cycles must be detected by visited-URL tracking
- `--rsh-max-pages` and `--rsh-max-items` are hard stops

Automatic pagination should never become an accidental infinite loop.

`--rsh-max-items` is a hard stop in both collected and streaming pagination.
Once the item limit is reached, the paginator stops without rendering a final
empty page or trailer value.

## Output Contracts

Two output contracts matter here:

- **document output** preserves one logical response shape across all pages
- **record output** emits one item at a time for incremental processing

Pagination may change execution strategy, but it must not silently change the
meaning of an explicit output format.

In particular:

- `-o json` always produces one valid JSON document
- `-o yaml` always produces one valid YAML document
- `-o readable` keeps a coherent human-oriented framing contract
- `-o ndjson` is the explicit record format for one item per line

Design 028 defines the planner that combines pagination with filtering and
format-family selection.

## Shape Preservation

For paginated bounded responses, Restish should preserve the logical response
shape whenever possible:

- a bare paginated array stays an array in document formats
- a wrapped object with `items_path: data` stays an object whose `data` field
  becomes the merged collection
- paginated filtering in document mode operates on the logical merged shape, not
  on one page at a time

If `items_path` is configured, document-oriented pagination should preserve the
wrapper object and only merge the collection field, not flatten the entire
response into an array by accident.

## Page-Follow Algorithm

The conceptual paginator algorithm is:

1. fetch first page
2. normalize response and extract links
3. determine whether pagination applies
4. emit or collect items from the current page according to the output plan
5. resolve the next target from:
   - normalized `next` relation, or
   - configured `next_path`
6. stop when:
   - there is no next target
   - a safety bound is reached
   - the context is canceled
   - the planner no longer permits more pages
7. merge or stream the final logical result according to the selected output
   family

This algorithm should remain explicit in the implementation rather than hidden
inside formatter logic or link parsers.

## Observability

Pagination is one of the places where verbose output is especially useful.
Diagnostics should be able to show:

- whether pagination activated
- which next URI was chosen
- current page number / item counts
- why pagination stopped

This helps users debug both hypermedia parsing and pagination config.

Pagination diagnostics are diagnostics, not response data. Page progress such
as "fetching page 2" belongs on stderr, should usually be gated by verbose mode
or rendered as terminal status, and must never appear in stdout between records
or document fragments.

## Examples

A response with a standard next link:

```http
HTTP/2 200 OK
Link: <https://api.example.com/items?page=2>; rel="next"
Content-Type: application/json

[1,2,3]
```

produces a normalized link map like:

```json
{
  "next": "https://api.example.com/items?page=2"
}
```

and allows:

```bash
restish get https://api.example.com/items
```

to produce one merged logical result across all pages by default on non-TTY
output, while `-o ndjson` streams records explicitly.

For an API with nested collection data, config can guide pagination:

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "pagination": {
        "items_path": "data"
      }
    }
  }
}
```

so a body like:

```json
{
  "data": [1, 2, 3],
  "meta": {"total": 3}
}
```

is treated as a three-item page rather than a single object item.

Collection-oriented filtering still works through collect-style execution:

```bash
restish get https://api.example.com/items --rsh-collect -f '.body | length'
```

## Alternatives Considered

### Do Not Paginate Automatically

Simpler, but too much repetitive work for users.

### Hard-Code Pagination Per API Style

Does not scale and mixes too much policy into the request pipeline.

### Always Collect All Pages Before Output

Simpler for formatting, but too expensive for latency and memory.

## Relationship To Other Designs

- Design 009 defines the normalized response model that carries discovered
  links.
- Design 012 distinguishes true streams from bounded paginated collections.
- Design 015 exposes normalized links directly through the `links` command.
- Design 028 defines how pagination composes with output-family planning.
