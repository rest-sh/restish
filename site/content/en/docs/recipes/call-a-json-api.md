---
title: Call a JSON API
linkTitle: Call a JSON API
weight: 10
description: Ask an API for JSON while keeping interactive output readable.
---

```bash
restish -H 'Accept: application/json' https://api.rest.sh/get
```

Representative output:

```readable
method: "GET"
path: "/get"
```

Add query params with `-q`:

```bash
restish -H 'Accept: application/json' -q active=true https://api.rest.sh/get
```

Add `-o json` when the next command needs a JSON document.

For repeated headers or auth, put them in a profile instead of repeating them.

Related: [Requests](/docs/guides/requests/), [Content Types](/docs/reference/content-types/).
