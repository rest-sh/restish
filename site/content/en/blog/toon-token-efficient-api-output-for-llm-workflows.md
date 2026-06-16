---
title: "TOON: Token-Efficient API Output for LLM Workflows"
linkTitle: "TOON: Token-Efficient API Output for LLM Workflows"
date: 2026-06-15
author: "Daniel Taylor"
description: Restish can now render API responses as TOON, a compact text encoding of the JSON data model that cuts token costs when an LLM agent reads the result. Here is where it wins, where it loses, and how to combine it with filtering.
canonical_url: "https://rest.sh/blog/toon-token-efficient-api-output-for-llm-workflows/"
categories:
  - AI
tags:
  - ai
  - llm
  - cli
  - devtools
---

When a person reads an API response, formatting is free. When an LLM agent
reads one, every character is metered. The response lands in a context window,
the context window is billed by the token, and the format you chose decides how
many tokens the same data costs.

JSON spends a lot of tokens on that job. A list of one hundred records repeats
every key one hundred times, wraps every string in quotes, and spends tokens on
braces and brackets that a model does not need to understand a table of data.

Restish — a CLI for REST-ish HTTP APIs that can make one-off HTTP requests or
turn OpenAPI descriptions into shell-native commands — now has an output format
aimed at exactly this situation. `-o toon` renders the response as
[TOON](https://github.com/toon-format/spec) (Token-Oriented Object Notation), a
compact, lossless text encoding of the JSON data model. It was contributed by
[Omer Amar](https://github.com/omer-amar), and it slots into the same output
pipeline as JSON, YAML, tables, and NDJSON.

This post shows what TOON does to a response, how to pair it with filtering and
pagination, and when you should not use it.

<div class="restish-blog-callout">
  <strong>Try it as you read.</strong> Runnable examples below use the browser
  preview against the public <code>api.rest.sh</code> API. Local setup commands
  are shown as fenced shell snippets.
</div>

## What TOON Does to a List

Here is a small image collection rendered as JSON:

{{< restish-example >}}
restish api.rest.sh/images -o json
{{< /restish-example >}}

Every record repeats `format`, `name`, and `self`. Now the same response as
TOON:

{{< restish-example >}}
restish api.rest.sh/images -o toon
{{< /restish-example >}}

The header `[5]{format,name,self}:` declares the row count and the field names
once. After that, each record is one row. No repeated keys, no braces, and
quotes only where a value actually needs them. The encoding is still lossless:
types, nulls, and special characters survive the round trip, which is why the
header line looks fussier than a plain CSV.

That collapsed form — TOON calls it a tabular array — is where the savings
live. It applies when an array holds uniform, flat objects whose values are
primitives. Five records barely matter; one hundred records of repeated keys is
where JSON starts billing you for the same field names over and over.

## Filter First, Then Encode

Re-encoding is the second-biggest savings. The biggest is not sending data the
model never needed.

Restish filters run before output formatting, so you can project a response
down to the fields that matter and then let TOON collapse what remains:

{{< restish-example >}}
restish api.rest.sh/images -f 'body.{name, format}' -o toon
{{< /restish-example >}}

This combination matters more than either half alone. Filtering drops whole
fields, which saves more tokens than any re-encoding can. And projecting to a
uniform list of primitives is exactly what keeps TOON in its tabular form —
flat, uniform records keep the output collapsed instead of falling back to a
more verbose nested layout.

A good habit for agent-facing commands: decide which fields the model needs,
write the filter, then add `-o toon`.

## Pagination Is Already One Table

There is a detail hiding in the examples above: `api.rest.sh/images` is a
paginated endpoint, and those five records arrived across multiple pages.
Restish followed the `next` links automatically, and document formats like
TOON gather every page into one body before rendering — so a multi-page
collection still comes out as a single table with one header line. No flag
required, and no per-page overhead reaches the model.

Filters work across pages too: without extra flags they run once per paginated
item, which is why `body.{name, format}` projected every record above. Reach
for `--rsh-collect` only when a filter needs to see the whole collection at
once — counting items, for example. The
[pagination guide](/docs/guides/pagination/) covers limits, link following,
and collect semantics.

## The Numbers

Token counts for the same data rendered in each format, counted with
`o200k_base` (GPT-4o-class tokenizer). "Uniform 100" is a 100-row record
collection; "Nested 40" is a collection of nested, irregular objects:

| Format         | Uniform 100 | Nested 40 |
| -------------- | ----------: | --------: |
| **toon**       |   **1,689** | **3,343** |
| json (compact) |       2,903 |     2,762 |
| json (pretty)  |       5,002 |     4,842 |
| yaml           |       3,700 |     3,360 |
| ndjson         |       3,000 |     2,800 |
| gron           |       6,403 |     6,783 |

On the uniform collection, TOON costs about 42% fewer tokens than compact JSON
and about two-thirds less than pretty-printed JSON — and the lead grows with
row count, because the per-record overhead is what TOON eliminates.

The second column is where the tradeoff shows, though, and it deserves its own
section.

## Where TOON Loses

On nested, irregular data, compact JSON and NDJSON beat TOON. When records do
not share a flat shape, TOON falls back to an indented layout, and the
per-line indentation costs more than the removed punctuation saves. You can see
the shape change on a nested response:

{{< restish-example >}}
restish api.rest.sh/example -f body.basics -o toon
{{< /restish-example >}}

Still readable, and the embedded `profiles` array still collapses into a table.
But for deeply nested or irregular data, this layout stops being a token win.
The rule of thumb: project to a flat, primitive-valued list first, or stay on
JSON.

Two more tradeoffs worth stating plainly:

- **TOON is output-only.** Restish renders it but does not accept TOON request
  bodies. JSON remains the interchange format for anything that talks back to
  an API.
- **Token savings only pay off if your model parses TOON as reliably as
  JSON.** Models have seen vastly more JSON than TOON in training. The flat
  tabular form is simple, but validate against your own model and your own
  data before making it a default.

## Where This Fits

The obvious question: if the goal is giving an agent API access, why not an MCP
server?

MCP is the right answer for many setups, and Restish
[can serve registered APIs as MCP tools](/docs/plugins/mcp/). But
a large amount of real agent work happens through plain shell commands — a
coding agent that can run CLI tools already has everything it needs to call an
API through Restish. In that mode, Restish is the tool surface: the agent runs
a command, and stdout goes straight into its context.

That is the niche `-o toon` serves. The agent gets the same request pipeline a
human gets — profiles, auth, TLS, retries, pagination, normalization,
filtering — and the response arrives in its context at a lower token price.
The encoder plugs into the same formatter pipeline as the built-in formats, so
TOON composes with `-f` filters and paginated values like any other `-o`
choice, and it added no new dependencies to the binary.

A practical pattern for an agent-callable script:

```bash
restish myapi list-orders --status open -f 'body.{id, customer, total}' -o toon
```

One line, and the spec-derived command, the credential handling, and the token
budget are all handled.

## Try It

Install Restish:

```bash
brew install restish
restish --version
```

Or with Go:

```bash
go install github.com/rest-sh/restish/v2/cmd/restish@latest
restish --help
```

Then render something as TOON:

```bash
restish api.rest.sh/images -f 'body.{name, format}' -o toon
```

Useful next stops:

- [Output Formats Reference](/docs/reference/output-formats/#toon-for-agents)
  covers the full TOON tradeoffs and benchmark details.
- [Filtering Guide](/docs/guides/filtering/) explains projections like
  `body.{name, format}`.
- [Output Guide](/docs/guides/output/) covers the processing model and
  document-versus-record output.
- [Pagination Guide](/docs/guides/pagination/) explains link following,
  limits, and `--rsh-collect`.

Formats are not neutral when the reader is billed by the token. Filter to what
the model needs, collapse the rest with TOON, and spend the saved context on
something useful.
