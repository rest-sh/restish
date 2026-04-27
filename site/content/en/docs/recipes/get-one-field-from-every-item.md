---
title: Get One Field From Every Item
linkTitle: Get One Field
weight: 35
description: Print one field per collection item.
---

```bash
restish https://api.rest.sh/images -f body.self -r
```

Example output:

```text
/images/jpeg
/images/webp
/images/gif
/images/png
/images/heic
```

Use raw mode for shell-friendly scalar output.

Related: [Filtering](/docs/guides/filtering/).
