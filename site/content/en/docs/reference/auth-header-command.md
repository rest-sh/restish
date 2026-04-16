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

## What It Is Good For

Use `auth-header` when:

- you want to confirm which profile is active
- you need to verify that token acquisition succeeded
- you want to inspect a header value without sending a live request

This is especially useful with profile-specific auth, `external-tool` auth, or
OAuth flows that cache tokens.

## Common Workflow

1. configure an API and profile
2. run `restish auth-header <api>`
3. compare the emitted header with what the target service expects
4. switch profiles with `-p` if you need to verify another environment

If auth resolution fails, treat it the same way you would treat a failed real
request: check the profile, auth params, cached credentials, and plugin setup.

## Related Pages

- [Authentication](/docs/guides/authentication/)
- [API Management](../api-management/)
- [Profiles](../profiles/)
