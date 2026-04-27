---
title: Show Only Response Headers
linkTitle: Show Headers
weight: 38
description: Print response headers without the body.
---

```bash
restish https://api.rest.sh/ --rsh-headers
```

Filter one header as raw text:

```bash
restish https://api.rest.sh/ -f headers.Content-Type -r
```

Use `/headers` when you want the request headers the server received:

```bash
restish https://api.rest.sh/headers
```

Related: [Output](/docs/guides/output/), [Global Flags](/docs/reference/global-flags/).
