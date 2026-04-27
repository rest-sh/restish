---
title: Save a Response Unchanged
linkTitle: Save Unchanged
weight: 40
description: Save original response bytes instead of formatted output.
---

```bash
restish https://api.rest.sh/bytes/64 --rsh-raw > sample.bin
```

For an image:

```bash
restish https://api.rest.sh/images/jpeg > dragonfly.jpg
```

Image responses redirect as original bytes by default. Use `--rsh-raw` for byte
streams that would otherwise be decoded or formatted.

Related: [Output](/docs/guides/output/).
