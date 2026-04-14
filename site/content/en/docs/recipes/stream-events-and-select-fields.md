---
title: Stream Events and Select Fields
linkTitle: Stream Events and Select Fields
weight: 60
description: Stream SSE or NDJSON responses and extract just the fields you care about.
---

For SSE or NDJSON endpoints, stream the response and filter each event:

```bash
restish https://api.example.com/events -f '.body.type' -r
restish https://api.example.com/logs -f '.body.user.id' -r
```

Use `--rsh-max-events` when you want only the first few items:

```bash
restish https://api.example.com/events --rsh-max-events 10 -f '.body.message' -r
```
