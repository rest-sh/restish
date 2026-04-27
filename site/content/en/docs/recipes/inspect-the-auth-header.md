---
title: Inspect the Auth Header
linkTitle: Inspect Auth Header
weight: 72
description: Print the Authorization header Restish would send for a configured API.
---

```bash
restish auth-header example
```

Then verify a bearer token against the safe fixture:

```bash
restish -H 'Authorization: Bearer docs-token' https://api.rest.sh/auth/bearer
```

Use verbose mode when you need to inspect all request headers.

Related: [Authentication](/docs/guides/authentication/), [Auth Header Command](/docs/reference/auth-header-command/).
