---
title: Save a Response Unchanged
linkTitle: Save a Response Unchanged
weight: 40
description: Save the original response body bytes to disk with Restish.
---

Use raw output when you want the original response body bytes on disk.

```bash
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
```

That works because non-TTY output defaults to `raw`.

If you want to make the intent explicit:

```bash
restish https://api.rest.sh/images/jpeg -o raw > dragonfly.jpg
```

Do not add filters if you need byte-for-byte preservation. Filters operate on
the normalized decoded response, so the result is no longer the original wire
payload.
