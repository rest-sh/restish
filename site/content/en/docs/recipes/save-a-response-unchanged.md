---
title: Save a Response Unchanged
linkTitle: Save Unchanged
weight: 40
description: Save original response bytes instead of formatted output.
---

```bash
restish https://api.rest.sh/bytes/64 -o raw > sample.bin
```

For an image:

```bash
restish https://api.rest.sh/images/jpeg -o raw > dragonfly.jpg
```

Use `-o raw` whenever byte fidelity matters.

Related: [Output](/docs/guides/output/).
