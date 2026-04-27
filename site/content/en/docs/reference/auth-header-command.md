---
title: Auth Header Command
linkTitle: Auth Header
weight: 40
description: Print the Authorization header value for a configured API.
---

`auth-header` answers a narrow debugging question: "what `Authorization` header
would Restish send for this API and profile?" It resolves configured auth
without making the target API request, which keeps credential debugging separate
from server behavior.

## Examples

```bash
restish auth-header example
restish -p staging auth-header example
```

## Notes

Use this to confirm profile auth before debugging a `401` or `403`. If no
Authorization header is configured, the command reports that clearly. For
headers outside `Authorization`, use verbose mode against a safe endpoint such
as `https://api.rest.sh/headers`.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Authentication](/docs/guides/authentication/)
- [Profiles](/docs/reference/profiles/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
