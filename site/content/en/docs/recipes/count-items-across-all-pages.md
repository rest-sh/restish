---
title: Count Items Across All Pages
linkTitle: Count Across Pages
weight: 55
description: Collect a paginated response into one logical result and count the items.
---

When the server paginates a collection, collect the full logical result before
counting:

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

Example output:

```text
5
```

Use `--rsh-collect` when the filter needs the whole collection instead of
processing one item at a time.
