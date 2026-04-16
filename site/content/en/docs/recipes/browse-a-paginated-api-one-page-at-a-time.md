---
title: Browse a Paginated API One Page at a Time
linkTitle: One Page at a Time
weight: 76
description: Disable automatic pagination when you want to inspect just the first page and its next link.
---

Use:

```bash
restish https://api.rest.sh/images --rsh-no-paginate
```

This returns only the first page and leaves the discovered `next` link visible
for inspection.
