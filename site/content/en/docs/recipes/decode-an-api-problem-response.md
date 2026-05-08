---
title: Decode an API Problem Response
linkTitle: API Problem
weight: 46
description: Inspect an application/problem+json response.
---

Many APIs return structured error bodies using the `application/problem+json`
media type. Restish decodes that body like other JSON-family responses, but a
non-2xx status normally makes the command fail. Use this recipe when the error
document itself is the thing you need to inspect.

```bash
restish api.rest.sh/problem --rsh-ignore-status-code
```

Use `--rsh-ignore-status-code` so the response body remains visible even though
the HTTP status is an error. Add `-o json` when a script needs the problem
document as JSON. The [Content Types reference](/docs/reference/content-types/)
explains why `+json` response types decode automatically.

Related: [Content Types](/docs/reference/content-types/), [Command Behavior](/docs/guides/command-behavior/).
