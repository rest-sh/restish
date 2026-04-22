# Links Command

## Summary

Restish v2 includes a `links` command that performs a GET request, runs the
hypermedia parsers, and prints the discovered links directly as JSON.

This provides a focused tool for inspecting link relations without needing to
manually filter full response output.

## Goals

- expose normalized hypermedia links directly to users
- provide a compact debugging tool for pagination and hypermedia behavior
- keep the output aligned with Restish's internal normalized link model
- avoid requiring users to understand or reproduce parser internals manually

## Non-Goals

- becoming a general-purpose alternate output formatter
- exposing raw wire link syntax instead of Restish's normalized interpretation
- bypassing the normal request/auth/TLS pipeline

## Position In The Architecture

The `links` command is a focused wrapper around the normal request and
hypermedia pipeline.

It exists because hypermedia parsing is useful both:

- internally for pagination and response inspection
- externally for human debugging and API exploration

## Command Flow

The `links` command works conceptually like this:

1. perform a GET against the target URI or API-relative path
2. normalize the response enough for hypermedia parsing
3. run all registered hypermedia parsers
4. merge the discovered links into the normalized `rel -> uri` map
5. optionally filter that map by relation names passed on the command line
6. print the final relation map as JSON

The command intentionally does less than the general response path. It is not
trying to be a formatter mode; it is a targeted inspection command for the
link-normalization layer.

## Input Resolution

Because the command goes through the normal request path, it should still honor:

- API registrations
- profiles
- auth
- TLS settings
- retry and cache behavior where appropriate

This is important because users often need to inspect links on the same secured
APIs they use with normal requests.

## Output Contract

The output is the normalized relation map:

- relation name -> absolute URI

This is a deliberate product choice. The command shows Restish's interpreted
link model, not the raw wire representation from headers or body documents.

That makes it the right companion for debugging:

- pagination activation
- body-parser link extraction
- relation naming and normalization

## Relation Filtering

Users may supply one or more relation names after the URI to filter output down
to only those relations.

This is intentionally simple:

- no jq here
- no partial matching
- no alternate output modes

The command is for quick inspection, not data transformation.

## Failure Model

The command should distinguish between:

- request failure
- no links discovered
- requested relation names not present

An empty JSON object is a meaningful result when no matching relations were
found. That is different from transport failure.

## Why It Exists Separately

Users could technically extract links from full response output with filtering,
but a dedicated command is still valuable because it:

- shortens a common debugging task
- presents the exact normalized link layer Restish uses internally
- avoids requiring users to remember the full normalized response layout

This is especially useful when debugging why pagination did or did not activate.

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

### Expect Users To Filter Links Out Of Full Response Output

Possible, but clumsier and less explicit for a common task.

### Treat Links As Only An Internal Detail

Would make pagination and hypermedia behavior much harder to debug.

## Relationship To Other Designs

- Design 011 defines the normalized link model this command exposes.
- Design 029 defines the shared request pipeline it reuses.
- Design 017 defines the stdout/stderr and exit-behavior expectations around
  command output.
