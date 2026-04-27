---
title: Decode an API Problem Response
linkTitle: API Problem
weight: 46
description: Inspect an application/problem+json response.
---

```bash
restish https://api.rest.sh/problem --rsh-ignore-status-code
```

Use `--rsh-ignore-status-code` so the response body remains visible even though
the HTTP status is an error. Add `-o json` when a script needs the problem
document as JSON.

Related: [Content Types](/docs/reference/content-types/), [Command Behavior](/docs/guides/command-behavior/).
