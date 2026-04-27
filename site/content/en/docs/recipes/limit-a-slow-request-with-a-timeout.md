---
title: Limit a Slow Request With a Timeout
linkTitle: Timeout Slow Request
weight: 62
description: Bound a slow request with --rsh-timeout.
---

```bash
restish 'https://api.rest.sh/slow?delay=2s' --rsh-timeout 500ms
```

Use a larger timeout when the delay is expected:

```bash
restish 'https://api.rest.sh/slow?delay=2s' --rsh-timeout 3s
```

Related: [Retries and Caching](/docs/guides/retries-and-caching/), [Global Flags](/docs/reference/global-flags/).
