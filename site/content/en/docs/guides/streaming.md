---
title: Streaming
linkTitle: Streaming
weight: 90
description: Work with streaming responses such as SSE and NDJSON in Restish.
---

Restish supports streaming workflows, including event-based and line-oriented
response streams.

## Supported Stream Types

Restish currently recognizes:

- Server-Sent Events with `text/event-stream`
- newline-delimited JSON with `application/x-ndjson`
- JSON Lines with `application/jsonlines`

When one of those content types is detected, Restish switches to a per-item
pipeline instead of waiting for the whole response body.

## Basic Usage

For SSE:

```bash
restish https://api.example.com/events
```

For NDJSON:

```bash
restish https://api.example.com/logs
```

Each event or line is emitted as it arrives.

For explicit machine-oriented record output, prefer `ndjson`:

```bash
restish https://api.example.com/events -o ndjson
```

Think about stream handling in two categories:

- bounded streams, where EOF arrives naturally
- live streams, where the feed may continue indefinitely

## Filter Each Event

Filtering still works in streaming mode. Each event payload becomes `body` for
the filter expression:

```bash
restish https://api.example.com/events -f '.body.type'
restish https://api.example.com/events -f '.body.user.id'
```

This keeps streaming consistent with the rest of the CLI instead of inventing a
separate query model.

With `-o ndjson`, each filtered result is still one valid JSON value per line:

```bash
restish https://api.example.com/events -o ndjson -f '.body.user.id'
```

## Limit The Stream

Use `--rsh-max-events` to stop after a bounded number of items:

```bash
restish https://api.example.com/events --rsh-max-events 10
```

This works for both SSE and NDJSON streams.

## Raw Stream Output

Use `-r` or `--rsh-raw` when you want shell-friendly scalar output from a
filtered stream:

```bash
restish https://api.example.com/events -f '.body.message' -r
```

That prints one result per event without JSON string quotes.

## Document Formats On Live Streams

True streams may be unbounded, so document formats such as `json` are not a
good fit.

Restish treats `-o json` as a bounded-document request and returns a clear
error for live streams:

```bash
restish https://api.example.com/events -o json
```

Use `-o ndjson` when you want structured streaming JSON instead.

That is the practical reason `json` and live streams do not mix well: one asks
for a full document, while the other may never finish.

## How SSE Events Are Parsed

For SSE responses:

- multiple `data:` lines are joined into one event payload
- a blank line ends the event
- `id` and `event` fields are currently ignored for output
- `retry` is parsed as a hint, but automatic reconnect is not implemented

## Related Guides

- [Filtering](../filtering/)
- [Output](../output/)
- [Output Formats](/docs/reference/output-formats/)
