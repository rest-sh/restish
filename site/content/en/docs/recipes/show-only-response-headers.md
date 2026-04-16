---
title: Show Only Response Headers
linkTitle: Show Only Headers
weight: 35
description: Print just the response headers from a Restish request.
---

Use `--rsh-headers` when you only want the normalized response headers:

```bash
restish https://api.rest.sh/ --rsh-headers
```

This is equivalent to:

```bash
restish https://api.rest.sh/ -f headers
```

For shell-friendly access to one header:

```bash
restish https://api.rest.sh/ -f headers.Content-Type -r
```
