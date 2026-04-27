---
title: Send a Form Login Request
linkTitle: Form Login
weight: 21
description: Send application/x-www-form-urlencoded data.
---

```bash
restish post -c form https://api.rest.sh/login 'username: alice, password: secret'
```

Representative output:

```json
{
  "token": "docs-token-alice",
  "token_type": "Bearer",
  "user": "alice"
}
```

Related: [Input and Shorthand](/docs/guides/input/), [Content Types](/docs/reference/content-types/).
