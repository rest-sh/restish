---
title: Auth Header Command
linkTitle: Auth Header Command
weight: 16
description: Reference for restish auth-header, which shows the Authorization header Restish would send.
---

`restish auth-header <api>` prints the resolved `Authorization` header for the
selected API and profile.

## Examples

```bash
restish auth-header example
restish -p ci auth-header example
```

This uses the same auth resolution path as a real request, which makes it one
of the best debugging commands for profile-driven auth.

## Related Pages

- [Authentication](/docs/guides/authentication/)
- [API Management](../api-management/)
