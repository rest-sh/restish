---
title: Stream Events and Select Fields
linkTitle: Stream Fields
weight: 60
description: Read a bounded stream and print selected event fields.
---

Streams may not have a natural end. Always bound examples with
`--rsh-max-events` unless you intentionally want to keep listening. Filters let
you select one field from each event as it arrives.

```bash
restish https://api.rest.sh/events --rsh-max-events 3 -f data.type -o lines
restish https://api.rest.sh/events --rsh-max-events 3 -f data.user.id -o lines
```

Use NDJSON output for structured event records:

```bash
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

Use scalar filters with `-o lines` when humans or shell loops need one value
per line. Use `-o ndjson` when the next tool expects structured event records.
The
[Streaming guide](/docs/guides/streaming/) explains why JSON document output is
not a good fit for unbounded streams.

Related: [Streaming](/docs/guides/streaming/).
