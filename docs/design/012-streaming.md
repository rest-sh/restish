# Streaming

## Summary

Restish v2 treats streaming response formats as a distinct execution path.
Instead of waiting for full response normalization, it reads events incrementally
and applies filtering/output logic per event as data arrives.

## Problem

Some APIs return open-ended or large incremental streams rather than bounded
response bodies. Waiting to fully buffer those responses would break the main
benefit of streaming: seeing items as they arrive.

The streaming design needed to:

- recognize common stream formats
- emit output incrementally
- allow filters to apply to each event
- keep stream output predictable for shell use
- preserve a simple mental model despite the different execution path

## Design

Restish currently recognizes two stream shapes:

- Server-Sent Events via `text/event-stream`
- newline-delimited JSON via `application/x-ndjson` and `application/jsonlines`

When a response is identified as streaming, Restish bypasses the normal
full-body normalization path and instead processes one event or line at a time.

Streaming classification happens after headers arrive. This matters because
streaming requests must not depend on whole-request `http.Client.Timeout`; they
need timeout behavior that allows long-lived bodies once headers have been
received.

For each stream item:

1. parse the item as JSON when possible
2. fall back to plain string data when JSON parsing fails
3. apply the configured filter against a document whose `body` is that item
4. write the result immediately

This keeps the stream path aligned with the rest of the CLI model: filters still
work, `--rsh-raw` still affects display, and stdout still receives one logical
result per emitted event.

Because true streams may be unbounded, explicit document formats are not always
meaningful. Restish therefore treats `-o json` as a bounded-document request
and rejects it for streaming responses, pointing users to `-o ndjson` instead.

Readable output remains valid for streams, but it is an incremental human view,
not one coherent machine document.

There are a few design choices worth preserving:

- SSE joins multiple `data:` lines into one event payload
- blank lines terminate SSE events
- `--rsh-max-events` provides a simple safety limit for both SSE and NDJSON
- stream reads treat end-of-stream and many scanner errors as normal completion
  rather than fatal failures
- context cancellation should interrupt stream reads promptly

## Examples

An SSE stream like:

```text
data: {"n":1}

data: {"n":2}

data: {"n":3}
```

can be read with:

```bash
restish get https://api.example.com/events
```

and emits one output line per event.

Per-event filtering works the same way:

```bash
restish get https://api.example.com/events -f '.body.type'
```

For NDJSON:

```text
{"n":1}
{"n":2}
{"n":3}
```

Restish again emits one result per line as the data arrives. Users who want
that behavior explicitly can choose:

```bash
restish get https://api.example.com/events -o ndjson
```

To stop after a bounded number of events:

```bash
restish get https://api.example.com/events --rsh-max-events 2
```

## Alternatives Considered

### Buffer the entire stream before output

This defeats the purpose of streaming and would behave poorly for open-ended or
high-volume feeds.

### Pretend document formats are stream-safe

That would make `-o json` ambiguous or invalid for live streams. It is better
for Restish to keep document formats strict and provide `ndjson` as the
explicit record-oriented JSON format.

### Reuse the full response normalization path unchanged

That path assumes a bounded body and a completed response. Streaming needs a
per-item pipeline instead.

### Disable filtering for stream responses

That would simplify implementation, but it would make streams much less useful.
Applying the same filter model per event gives users a much more consistent CLI.

## Notes

The current implementation reflects this design directly:

- `internal/cli/stream.go` detects stream content types and handles SSE or
  NDJSON incrementally
- filters are applied per item using the same filter package as normal
  responses
- output is written line-by-line for incremental consumption, with `ndjson` as
  the explicit record-oriented JSON formatter

One detail worth preserving is that stream filters operate on a document where
the event payload is exposed as `body`. That lets the same jq-style and
response-style filters remain useful without inventing a separate stream query
language.
