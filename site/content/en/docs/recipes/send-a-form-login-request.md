---
title: Send a Form Login Request
linkTitle: Form Login
weight: 21
description: Send application/x-www-form-urlencoded data.
---

Older login and token endpoints often expect `application/x-www-form-urlencoded`
instead of JSON. `-c form` chooses that request encoding while still letting you
use Restish shorthand for the fields.

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

The example API returns a safe demo token so you can see the shape without
using a real identity provider. For production APIs, store repeated auth
settings in a [profile](/docs/reference/profiles/) instead of retyping secrets.

Related: [Input and Shorthand](/docs/guides/input/), [Content Types](/docs/reference/content-types/).
