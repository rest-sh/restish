---
title: Streaming
linkTitle: Streaming
weight: 90
description: Work with SSE and NDJSON streams while keeping output bounded and script-friendly.
---

Restish recognizes Server-Sent Events, NDJSON, and JSON Lines. Streaming
responses are processed one event or line at a time instead of waiting for a
complete response body.

## Server-Sent Events

Always bound examples you paste into a terminal:

```bash
restish https://api.rest.sh/events --rsh-max-events 3
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

SSE output includes event metadata and parsed data. Filter the event data when
you only need fields:

```bash
restish https://api.rest.sh/events --rsh-max-events 3 -f data.type -o lines
restish https://api.rest.sh/events --rsh-max-events 3 -f data.user.id -o lines
```

## NDJSON

The `/logs` endpoint emits line-oriented JSON records:

```bash
restish https://api.rest.sh/logs --rsh-max-events 3 -o ndjson
restish https://api.rest.sh/logs --rsh-max-events 3 -f body.user.id -o lines
```

If an endpoint is slow to emit its first record, add a timeout while debugging:

```bash
restish https://api.rest.sh/logs --rsh-max-events 3 --rsh-timeout 5s
```

Very large NDJSON records use the same per-response cap as bounded responses.
Raise it with `--rsh-max-body-size` when a stream legitimately emits large
single-line records.
SSE uses the same line cap and also caps the accumulated `data:` payload for one
event, including multi-line events.

## Accept Headers

When a server needs a stream-specific `Accept` header, send it explicitly:

```bash
restish -H 'Accept: text/event-stream' https://api.rest.sh/events --rsh-max-events 3
```

## Document Formats On Live Streams

Document formats such as `json` and `yaml` require one complete document. For
live streams, prefer `ndjson` for structured records or `lines` for filtered
scalar values:

```bash
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
restish https://api.rest.sh/events --rsh-max-events 3 -f data.message -o lines
```

## SSE Parsing Notes

- multiple `data:` lines are joined into one event payload
- a blank line ends an event
- lines starting with `:` are comments
- a field without `:` has an empty value
- event metadata is preserved in the normalized event output
- automatic reconnect is not a replacement for application-level retry logic

## Related Pages

- [Output](../output/)
- [Filtering](../filtering/)
- [Output Formats](/docs/reference/output-formats/)
- [Stream Events and Select Fields](/docs/recipes/stream-events-and-select-fields/)
