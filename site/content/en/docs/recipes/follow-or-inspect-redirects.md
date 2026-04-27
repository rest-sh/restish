---
title: Follow or Inspect Redirects
linkTitle: Redirects
weight: 63
description: Use verbose output to inspect redirect behavior.
---

```bash
restish https://api.rest.sh/redirect/2 -v
```

Choose a redirect status and target:

```bash
restish 'https://api.rest.sh/redirect-to?url=/get&status_code=307' -v
```

Related: [Command Behavior](/docs/guides/command-behavior/).
