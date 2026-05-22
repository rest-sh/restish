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
- newline-delimited JSON via `application/x-ndjson`,
  `application/ndjson`, `application/jsonl`, and `application/jsonlines`

Streaming classification happens after headers arrive. This matters because
streaming requests must not depend on whole-request `http.Client.Timeout`; they
need timeout behavior that allows long-lived bodies once headers have been
received.

For v2, `--rsh-timeout` has split behavior. It can still bound the full
lifetime of ordinary, bounded responses. For SSE and NDJSON streams, it bounds
the wait for response headers; after headers identify a supported stream, body
reads are governed by root command cancellation and stream-specific limits such
as `--rsh-max-items` and `--rsh-max-body-size`.

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
work, `-o lines` can render filtered scalar values for shell pipelines, and
stdout still receives one logical result per emitted event.

If stdout is buffered for throughput, the stream path must flush at record or
event boundaries. Streaming should improve latency for users watching output
and should not leave piped consumers waiting for an arbitrary buffer to fill.

This guarantee starts once bytes reach Restish. A client cannot recover
low-latency behavior when an origin, CDN, reverse proxy, or compression layer
buffers the response until the full body is complete. Streaming examples and
tests should distinguish client-side flushing from server-side delivery by
using an endpoint that writes headers promptly, flushes each record or event,
and avoids a precomputed `Content-Length` for open-ended streams.

## SSE Semantics

SSE support should preserve the usual event framing rules:

- join multiple `data:` lines into one event payload
- treat blank lines as event boundaries
- treat a field without `:` as a field with an empty value
- treat lines beginning with `:` as comments
- parse `retry` with integer parsing and ignore invalid retry values
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

The streaming path uses the same recognized media types that the content
registry advertises for bounded NDJSON bodies. Bounded normalization may decode
the complete body into an array of records, but once the response is classified
as a stream the body must not be read to EOF before rendering the first line.

## Output Contracts

Because true streams may be unbounded, explicit document formats are not always
meaningful.

Restish therefore treats:

- `-o ndjson` as the explicit record-oriented JSON stream format
- `-o auto` as an incremental human view
- `-o json` as a bounded-document request that should reject clearly live
  streams unless paired with explicit collection and a finite item cap

Auto output remains valid for streams, but it is an incremental human view,
not one coherent machine document.

The accepted v2 escape hatch for JSON document output from a stream is:

```bash
restish get https://api.example.com/events --rsh-collect --rsh-max-items 10 -o json
```

That combination is intentionally verbose. `--rsh-collect` says the user wants
a document instead of incremental records, and `--rsh-max-items` gives Restish a
finite stop condition before building that document. Plain `-o json` on a live
stream should fail with a hint to use `-o ndjson` for record-by-record JSON.

## Stream Planner Rules

The stream planner should decide:

1. whether the response is a supported stream type
2. whether the selected output family can operate incrementally
3. whether the filter can be evaluated per event
4. whether the user requested a bounded stop such as `--rsh-max-items`
5. whether the requested format must fail because the stream is unbounded

This ties streaming into the same output-family model as design 028.

## Cancellation And Bounds

Streaming must remain interruptible and bounded when the user asks for it.

Important controls:

- context cancellation should interrupt stream reads promptly
- `--rsh-timeout` should not terminate a healthy stream only because the body
  has remained open past the header-wait deadline
- `--rsh-max-items` defaults to `0` and provides an explicit record limit for
  both SSE and NDJSON when the user wants bounded stream processing
- hitting `--rsh-max-items` is a successful bounded stop and must print a
  stderr warning naming the cap and `0` override
- SSE and NDJSON per-line reads are capped by the stream line limit derived
  from `--rsh-max-body-size`
- SSE accumulated `data:` payload for one event is capped separately by the
  same stream-size budget so many continuation lines cannot grow without bound
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

Users who need one bounded JSON fixture from a stream can opt into collection:

```bash
restish get https://api.example.com/events --rsh-collect --rsh-max-items 3 -o json
```

To stop after a bounded number of events:

```bash
restish get https://api.example.com/events --rsh-max-items 2
```

## Alternatives Considered

### Buffer The Entire Stream Before Output

Defeats the purpose of streaming.

This remains true even for finite stream-shaped responses. A response with
`application/x-ndjson` and a small `Content-Length` may be technically bounded,
but the stream path should still prefer line-at-a-time rendering when the
server delivers lines incrementally.

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
