---
title: Send an API Key in a Header
linkTitle: API Key Header
weight: 73
description: Call an endpoint that requires an X-API-Key header.
---

Header-based API keys are common because they keep credentials out of URLs,
logs, and browser history. Use `-H` for a one-off request, then move the header
into a profile when it becomes part of normal work.

```bash
restish -H 'X-API-Key: docs-key' https://api.rest.sh/auth/api-key-header
```

For repeated use, put the header in a [profile](/docs/reference/profiles/) or
use API configure prompts. The safe fixture accepts `docs-key` and echoes a
success response.

If you are not sure whether an API wants a header key, query key, bearer token,
or basic auth, check its OpenAPI security section or start with the broader
[Authentication guide](/docs/guides/authentication/).

Related: [Authentication](/docs/guides/authentication/), [Profiles](/docs/reference/profiles/).
