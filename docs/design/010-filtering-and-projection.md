# Filtering And Projection

## Summary

Restish v2 applies filtering after response normalization and before
formatting. Users can query the normalized response with either shorthand path
syntax or jq syntax, and Restish can usually infer which language they meant.

## Problem

Once Restish has a normalized response, users often want only part of it:

- just the body
- just a header
- a nested field
- a transformed or filtered list

The challenge is that different users want different levels of power. A simple
path expression is easier to learn and faster to type, while jq is much more
expressive for complex transformations.

The filtering design therefore needed to:

- expose the full normalized response, not just the body
- keep common cases lightweight
- preserve access to a more powerful query language when needed
- work predictably in both interactive and script-oriented modes

## Design

Filtering operates on a stable response document with these roots:

- `proto`
- `status`
- `headers`
- `links`
- `body`
- `@` for the full document

Restish supports two filter languages:

- shorthand path syntax for direct field access and simple selection
- jq for richer transformations and predicates

The default behavior is `auto`: if the expression begins with one of the known
normalized response roots, Restish treats it as shorthand; otherwise it treats
it as jq.

That gives users a convenient path for common queries like `body.name` or
`headers.Content-Type` without giving up jq for more advanced cases.

There are a few design choices worth preserving:

- filtering happens after normalization, so filters never depend on raw
  transport objects
- the default filter is context-sensitive: interactive output tends to start
  from the full response, while non-interactive output tends to start from the
  body
- `--rsh-raw` is a presentation shortcut layered on top of filtering, not a
  separate query language

`--rsh-raw` is intentionally narrow. It removes quotes from string results and
prints arrays of scalars one item per line, which makes common shell scripting
cases easier without inventing another general-purpose formatting mode.

Shorthand is intended for path-style queries over the normalized response
document. Typical expressions look like:

- `body.name`
- `body.items[0]`
- `headers.Content-Type`
- `links.next`

jq remains available for transformations and selections that go beyond direct
path access. The reference syntax is documented at
[jqlang.org/manual](https://jqlang.org/manual/).

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

Example raw output:

```bash
restish get https://api.example.com/items -f '.body.items[] | .name' --rsh-raw
```

which prints:

```text
alpha
beta
```

## Alternatives Considered

### Support only jq

This would be powerful, but it would raise the floor for common tasks. Restish
benefits from having a lightweight query path for simple inspection and
projection.

### Support only shorthand paths

This would be easier to explain, but it would cap the usefulness of filtering
too early. jq is worth keeping for transformations that go beyond field access.

### Filter only the body by default

That works for many API responses, but it hides useful protocol context. The
normalized response model is valuable precisely because filters can reach
headers, links, and status when needed.

## Notes

The current implementation reflects this design directly:

- `internal/filter/filter.go` resolves the filter language and applies either
  shorthand or jq
- `internal/filter/raw.go` implements the focused `--rsh-raw` display behavior
- `internal/cli/http.go` builds the normalized filter document and applies the
  filter before selecting a formatter

One subtle but important behavior is that filtering a sub-value changes what
non-TTY default output means. Once Restish is no longer working with the
original full response payload, it emits JSON for the filtered value rather than
trying to preserve raw bytes that no longer correspond to the selected result.
