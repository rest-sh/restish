---
title: Inspect the Auth Header
linkTitle: Inspect Auth Header
weight: 72
description: Print the Authorization header Restish would send for a configured API.
---

Authentication can come from profiles, prompts, OAuth flows, or external tools.
Before debugging a real API request, confirm what Restish would put in the
`Authorization` header for the selected API and profile.

```bash
restish auth-header example
```

Then verify a bearer token against the safe fixture:

```bash
restish -H 'Authorization: Bearer docs-token' https://api.rest.sh/auth/bearer
```

`auth-header` is focused on the credential header and avoids making the target
API call. Use verbose mode when you need to inspect all request headers. See
[Authentication](/docs/guides/authentication/) for the full profile and auth
model.

Related: [Authentication](/docs/guides/authentication/), [Auth Header Command](/docs/reference/auth-header-command/).
