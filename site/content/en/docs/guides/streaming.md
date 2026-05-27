---
title: Streaming
linkTitle: Streaming
weight: 90
description: Work with SSE and NDJSON streams while keeping output incremental and script-friendly.
aliases:
  - /docs/recipes/stream-events-and-select-fields/
---

Restish recognizes Server-Sent Events, NDJSON, and JSON Lines. Streaming
responses are processed one event or line at a time instead of waiting for a
complete response body.

Streams run until EOF, interruption, or an explicit stream limit. Add
`--rsh-max-items` when you want a sample or a script with a fixed record count.
For streams, `--rsh-timeout` bounds the wait for response headers; once headers
identify SSE or NDJSON, the body can stay open until EOF, Ctrl-C, or
`--rsh-max-items`.

## Server-Sent Events

Bound examples you paste into a terminal when you only want a sample:

{{< restish-example >}}
restish api.rest.sh/events --rsh-max-items 3
{{< /restish-example >}}

{{< restish-example >}}
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
{{< /restish-example >}}

SSE output includes event metadata and parsed data. Filter the event data when
you only need fields:

{{< restish-example >}}
restish api.rest.sh/events --rsh-max-items 3 -f body.data.type -o lines
{{< /restish-example >}}

```bash
restish api.rest.sh/events --rsh-max-items 3 -f body.data.user.id -o lines
```

## NDJSON

The `/logs` endpoint emits line-oriented JSON records:

{{< restish-example >}}
restish api.rest.sh/logs --rsh-max-items 3 -o ndjson
{{< /restish-example >}}

```bash
restish api.rest.sh/logs --rsh-max-items 3 -f body.user.id -o lines
```

If an endpoint is slow to respond with headers, add a timeout while debugging:

```bash
restish api.rest.sh/logs --rsh-max-items 3 --rsh-timeout 5s
```

Very large NDJSON records use the same per-response cap as bounded responses.
Raise it with `--rsh-max-body-size` when a stream legitimately emits large
single-line records.
SSE uses the same line cap and also caps the accumulated `data:` payload for one
event, including multi-line events.

## Accept Headers

When a server needs a stream-specific `Accept` header, send it explicitly:

{{< restish-example >}}
restish -H 'Accept: text/event-stream' api.rest.sh/events --rsh-max-items 3
{{< /restish-example >}}

## Output Formats On Live Streams

For live streams, prefer `ndjson` for structured records or `lines` for
filtered scalar values:

{{< restish-example >}}
restish api.rest.sh/events --rsh-max-items 3 -o ndjson
{{< /restish-example >}}

```bash
restish api.rest.sh/events --rsh-max-items 3 -f body.data.message -o lines
```

Some formats, including `yaml`, can render one streamed value at a time. Plain
`-o json` needs one valid JSON document, so use `--rsh-collect` together with a
finite `--rsh-max-items` when you explicitly want JSON array output from a
stream:

```bash
restish api.rest.sh/events --rsh-max-items 3 --rsh-collect -o json
```

Stream filters use the same mini response wrapper as per-item pagination: the
current event or NDJSON record is under `body`. For SSE, parsed event payload
fields live under `body.data`.

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
