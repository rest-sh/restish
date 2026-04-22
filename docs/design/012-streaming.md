# Streaming

## Summary

Restish v2 treats streaming response formats as a distinct execution path.
Instead of waiting for full response normalization, it reads events incrementally
and applies filtering/output logic per event as data arrives.

Streaming is the place where Restish most clearly diverges from its normal
"normalize full response then render" flow, so the contract needs to be
explicit.

## Goals

- recognize common stream formats
- emit output incrementally with low latency
- allow filters to apply to each event
- keep stream output predictable for shell use
- preserve a coherent distinction between bounded document output and live
  record output

## Non-Goals

- pretending every document formatter is meaningful for an unbounded stream
- buffering an open-ended stream into memory just to preserve the normal bounded
  path
- inventing a separate stream-only query language

## Stream Classification

Restish currently recognizes two stream shapes:

- Server-Sent Events via `text/event-stream`
- newline-delimited JSON via `application/x-ndjson` and `application/jsonlines`

Streaming classification happens after headers arrive. This matters because
streaming requests must not depend on whole-request `http.Client.Timeout`; they
need timeout behavior that allows long-lived bodies once headers have been
received.

## Stream Execution Path

When a response is identified as streaming, Restish bypasses the normal
full-body normalization path and instead processes one event or line at a time.

The conceptual execution loop is:

1. classify the response as a supported stream type
2. select a stream reader for that type
3. read one logical event/item
4. parse it into the event value model
5. expose that value as `body` in a per-event normalized document
6. apply filtering
7. render one logical output item
8. stop on EOF, context cancellation, or configured event limits

## Event Value Model

For each stream item:

1. parse the item as JSON when possible
2. fall back to plain string data when JSON parsing fails
3. apply the configured filter against a document whose `body` is that item
4. write the result immediately

This keeps the stream path aligned with the rest of the CLI model: filters still
work, `--rsh-raw` still affects display, and stdout still receives one logical
result per emitted event.

## SSE Semantics

SSE support should preserve the usual event framing rules:

- join multiple `data:` lines into one event payload
- treat blank lines as event boundaries
- ignore or minimally process non-`data:` fields unless and until Restish
  decides to expose a richer SSE event object model

The current design exposes the effective event payload through `body`, not the
full SSE wire event as a structured object.

## NDJSON Semantics

NDJSON support treats each line as one event/item.

When a line parses as JSON, the event value is that decoded JSON value.
Otherwise the event falls back to a plain string.

This is intentionally permissive because real-world newline-delimited streams
are not always perfectly homogeneous.

## Output Contracts

Because true streams may be unbounded, explicit document formats are not always
meaningful.

Restish therefore treats:

- `-o ndjson` as the explicit record-oriented JSON stream format
- `-o readable` as an incremental human view
- `-o json` as a bounded-document request that should reject clearly live
  streams

Readable output remains valid for streams, but it is an incremental human view,
not one coherent machine document.

## Stream Planner Rules

The stream planner should decide:

1. whether the response is a supported stream type
2. whether the selected output family can operate incrementally
3. whether the filter can be evaluated per event
4. whether the user requested a bounded stop such as `--rsh-max-events`
5. whether the requested format must fail because the stream is unbounded

This ties streaming into the same output-family model as design 028.

## Cancellation And Bounds

Streaming must remain interruptible and bounded when the user asks for it.

Important controls:

- context cancellation should interrupt stream reads promptly
- `--rsh-max-events` provides a simple safety limit for both SSE and NDJSON
- end-of-stream should be treated as normal completion

Stream reads should not hang indefinitely after the user canceled the command.

## Error Model

The stream path should distinguish between:

- normal EOF
- cancellation
- malformed event payload
- unsupported format choice

Malformed event payload should not necessarily kill the entire stream if the
chosen stream format allows plain-string fallback. But framing errors in the
stream protocol itself may still be fatal.

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

and emits one output line or record per event according to the selected output
mode.

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

### Buffer The Entire Stream Before Output

Defeats the purpose of streaming.

### Pretend Document Formats Are Stream-Safe

Would make `-o json` ambiguous or misleading for live feeds.

### Reuse The Full Response Normalization Path Unchanged

That path assumes bounded bodies and completed responses.

### Disable Filtering For Stream Responses

Would make the stream path much less useful.

## Relationship To Other Designs

- Design 009 defines the bounded-response normalization model this path bypasses.
- Design 010 defines the filter semantics reused per event.
- Design 013 affects timeout behavior at the transport layer before and after
  headers.
- Design 028 defines the output-family planner that determines which stream
  formats are valid.
