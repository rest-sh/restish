---
title: Streaming
linkTitle: Streaming
weight: 90
description: Work with streaming responses such as SSE and NDJSON in Restish.
---

# Streaming

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
restish get https://api.example.com/events
```

For NDJSON:

```bash
restish get https://api.example.com/logs
```

Each event or line is emitted as it arrives.

## Filter Each Event

Filtering still works in streaming mode. Each event payload becomes `body` for
the filter expression:

```bash
restish get https://api.example.com/events -f '.body.type'
restish get https://api.example.com/events -f '.body.user.id'
```

This keeps streaming consistent with the rest of the CLI instead of inventing a
separate query model.

## Limit The Stream

Use `--rsh-max-events` to stop after a bounded number of items:

```bash
restish get https://api.example.com/events --rsh-max-events 10
```

This works for both SSE and NDJSON streams.

## Raw Stream Output

Use `-r` or `--rsh-raw` when you want shell-friendly scalar output from a
filtered stream:

```bash
restish get https://api.example.com/events -f '.body.message' -r
```

That prints one result per event without JSON string quotes.

## How SSE Events Are Parsed

For SSE responses:

- multiple `data:` lines are joined into one event payload
- a blank line ends the event
- `id` and `event` fields are currently ignored for output
- `retry` is parsed as a hint, but automatic reconnect is not implemented

## Related Guides

- [Filtering](../filtering/)
- [Output](../output/)

Source material:

- [`docs/design/012-streaming.md`](/Users/daniel/src/restish2/docs/design/012-streaming.md)
