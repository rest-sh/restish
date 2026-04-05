# Links Command

## Summary

Restish v2 includes a `links` command that performs a GET request, runs the
hypermedia parsers, and prints the discovered links directly as JSON.

This provides a focused tool for inspecting link relations without needing to
manually filter full response output.

## Problem

Hypermedia parsing is useful internally for pagination and response inspection,
but users also benefit from seeing the discovered links directly:

- to understand navigable relations
- to debug hypermedia parsing
- to inspect APIs that advertise workflows through links

The design needed to expose link discovery in a compact, understandable form.

## Design

The `links` command is a focused wrapper around the normal request and
hypermedia pipeline:

1. GET the target URI
2. normalize the response body
3. run all registered hypermedia parsers
4. print the resulting `rel -> uri` map as JSON

Optionally, users can supply one or more relation names after the URI to filter
the output to only those links.

This command intentionally does less than the general response path. It is not
trying to be a formatter mode; it is a targeted inspection command for the
link-normalization layer.

## Examples

Show all discovered links:

```bash
restish links https://api.example.com/items
```

Filter to specific relations:

```bash
restish links https://api.example.com/items next self
```

Example output:

```json
{
  "next": "https://api.example.com/items?page=2",
  "self": "https://api.example.com/items?page=1"
}
```

## Alternatives Considered

### Expect users to filter links out of full response output

That is possible, but it is clumsier for a common debugging and inspection
task. A dedicated command is easier to teach and quicker to use.

### Treat links as only an internal implementation detail

That would make it harder to debug pagination and hypermedia handling. Exposing
the parsed link map directly is useful in its own right.

## Notes

The current implementation reflects this design directly:

- `internal/cli/links.go` implements the command
- it reuses the normal request, normalization, and hypermedia parsing pipeline
- output is a small JSON object keyed by relation name

One detail worth preserving is that the command shows the normalized link map,
not the raw wire representation. That keeps it aligned with how the rest of
Restish interprets hypermedia internally.
