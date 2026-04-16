---
title: Download a File With Headers Preserved
linkTitle: Download With Headers
weight: 42
description: Save a response body to disk while keeping the response headers visible in a separate file.
---

When you need both the original file bytes and the response headers, capture
them separately:

```bash
restish -v https://api.rest.sh/images/jpeg -o raw > dragonfly.jpg 2> dragonfly.headers.txt
```

This keeps:

- the raw response body in `dragonfly.jpg`
- verbose request and response metadata on stderr in `dragonfly.headers.txt`

Use `-o raw` when exact bytes matter.
