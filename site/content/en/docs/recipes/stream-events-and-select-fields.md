---
title: Stream Events and Select Fields
linkTitle: Stream Fields
weight: 60
description: Read a bounded stream and print selected event fields.
---

```bash
restish https://api.rest.sh/events --rsh-max-events 3 -f data.type -r
restish https://api.rest.sh/events --rsh-max-events 3 -f data.user.id -r
```

Use NDJSON output for structured event records:

```bash
restish https://api.rest.sh/events --rsh-max-events 3 -o ndjson
```

Related: [Streaming](/docs/guides/streaming/).
