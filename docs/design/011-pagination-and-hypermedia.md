# Pagination And Hypermedia

## Summary

Restish v2 treats link discovery and pagination as part of the normal response
pipeline. Hypermedia parsers extract normalized links from headers or response
bodies, and pagination builds on those links to follow multi-page collections
automatically.

## Problem

Many APIs spread collections across multiple pages, and they advertise the next
page in different ways:

- HTTP `Link` headers
- body-level hypermedia formats such as HAL or JSON:API
- API-specific fields that contain items or next-page URLs

Restish needed a pagination model that works well for common APIs without
forcing every user to write custom scripts for collection traversal.

## Design

The design has two layers.

First, hypermedia parsing normalizes discovered links into a simple `rel -> uri`
map. Built-in parsers currently recognize:

- HTTP `Link` headers
- HAL `_links`
- JSON-LD or TSJ `@id`
- JSON:API top-level `links`
- Siren `links`

All discovered links are resolved to absolute URLs before being stored. This
lets downstream behavior treat links uniformly regardless of how they were
represented on the wire.

Second, pagination uses that normalized link map plus optional per-API config.
For GET requests, if a `next` link is present, Restish can continue fetching
pages automatically.

Pagination is bounded by safety rules:

- context cancellation must be checked between pages
- cycles must be detected by visited-URL tracking
- `--rsh-max-pages` and `--rsh-max-items` are hard stops

Automatic pagination should never become an accidental infinite loop.

Two output contracts matter here:

- **document output** preserves one logical response shape across all pages
- **record output** emits one item at a time for incremental processing

Pagination may change execution strategy, but it should not silently change the
meaning of an explicit output format. In particular:

- `-o json` always produces one valid JSON document
- `-o yaml` always produces one valid YAML document
- `-o readable` keeps the same document framing on TTY, but may draw it
  incrementally as pages arrive
- `-o ndjson` is the explicit record format for one item per line

Per-API pagination config can refine how page data is interpreted:

- `items_path` extracts the collection from a nested body field
- `next_path` can extract a next-page URL from the body

If `items_path` is configured, document-oriented pagination should preserve the
wrapper object and only merge the collection field, not flatten the entire
response into an array by accident.

The overall intent is:

- make common paginated APIs work automatically
- preserve observability and escape hatches with flags like
  `--rsh-no-paginate`, `--rsh-max-pages`, `--rsh-max-items`, and
  `--rsh-collect`
- keep pagination layered on top of normalized responses instead of special
  casing specific APIs throughout the HTTP pipeline

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

### Do not paginate automatically

This would be simpler, but it would push a lot of repetitive paging logic onto
users for a very common API pattern.

### Hard-code pagination behavior per API style

That does not scale well and makes the request pipeline harder to maintain. A
normalized links layer plus small per-API overrides is more flexible.

### Always collect all pages before output

This would make downstream formatting simpler, but it would increase memory use
and delay first output for large collections. Record-oriented pagination still
matters for shell pipelines and exporter plugins, which is why Restish now has
an explicit `ndjson` formatter and readable incremental rendering on TTYs.

## Notes

The current implementation reflects this design directly:

- `internal/hypermedia/hypermedia.go` defines the normalized link model
- `internal/hypermedia/parsers.go` provides the built-in parsers
- `internal/cli/paginate.go` drives auto-pagination and output behavior

One detail worth preserving is that pagination keeps the logical response shape
when document formats are in use. A wrapped object with `items_path: data`
should still render as an object whose `data` field is the merged collection,
while explicit record formats such as `ndjson` may emit one item at a time.
