---
title: Call a JSON API
linkTitle: Call a JSON API
weight: 10
description: Ask an API for JSON while keeping interactive output readable.
---

Some APIs can return more than one representation. Restish sends a broad
`Accept` header by default, but you can ask a server to prefer JSON when that is
the clearest format for the task. The command still uses readable terminal
output, so the first result is easy to scan.

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

`-H` changes the HTTP request. `-q` adds query parameters without making you
quote a full URL. If the next command needs a JSON document, add `-o json`; for
interactive use, the readable default is usually easier.

For repeated headers or auth, put them in a [profile](/docs/reference/profiles/)
instead of repeating them.

Related: [Requests](/docs/guides/requests/), [Content Types](/docs/reference/content-types/).
