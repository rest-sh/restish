---
title: Ignore a 404 but Keep the Body
linkTitle: Ignore 404
weight: 50
description: Inspect an error response without a failing exit code.
---

```bash
restish https://api.rest.sh/status/404 --rsh-ignore-status-code
```

Use this when an error body is expected data for your script.

For problem details:

```bash
restish https://api.rest.sh/problem --rsh-ignore-status-code
```

Related: [Command Behavior](/docs/guides/command-behavior/).
