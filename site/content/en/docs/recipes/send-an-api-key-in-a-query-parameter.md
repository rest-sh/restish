---
title: Send an API Key in a Query Parameter
linkTitle: API Key Query
weight: 73
description: Call an endpoint that requires an api_key query parameter.
---

```bash
restish -q api_key=docs-key https://api.rest.sh/auth/api-key-query
```

Quoted URL form:

```bash
restish 'https://api.rest.sh/auth/api-key-query?api_key=docs-key'
```

Related: [Authentication](/docs/guides/authentication/), [Requests](/docs/guides/requests/).
