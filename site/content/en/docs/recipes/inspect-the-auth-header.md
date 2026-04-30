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
restish api auth inspect example --raw-header Authorization
```

Then verify a bearer token against the safe fixture:

```bash
restish -H 'Authorization: Bearer docs-token' https://api.rest.sh/auth/bearer
```

`api auth inspect` avoids making the target API call. Omit `--raw-header` for
redacted human-readable output, or add `--rsh-credential PartnerKey` to inspect
a named OpenAPI credential binding. Use verbose mode when you need to inspect
the whole request.

Related: [Authentication](/docs/guides/authentication/), [API Auth Inspect](/docs/reference/api auth inspect-command/).
