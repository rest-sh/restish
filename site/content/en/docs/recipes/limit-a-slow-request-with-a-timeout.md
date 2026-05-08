---
title: Limit a Slow Request With a Timeout
linkTitle: Timeout Slow Request
weight: 62
description: Bound a slow request with --rsh-timeout.
---

Timeouts keep an exploratory command from hanging forever and keep scripts from
waiting longer than their caller expects. Use a short timeout when you are
testing failure handling, and a realistic timeout when the slow response is
normal for the API.

```bash
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
```

Use a larger timeout when the delay is expected:

```bash
restish 'api.rest.sh/slow?delay=2s' --rsh-timeout 3s
```

The first command should fail quickly because the server waits longer than the
client allows. The second gives the fixture enough time to respond. Timeout and
retry settings often belong together; the [Retries and Caching guide](/docs/guides/retries-and-caching/)
explains how they interact.

Related: [Retries and Caching](/docs/guides/retries-and-caching/), [Global Flags](/docs/reference/global-flags/).
