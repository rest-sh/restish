---
title: Count Items Across All Pages
linkTitle: Count Across Pages
weight: 55
description: Collect a paginated response before counting items.
---

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

Example output:

```text
5
```

Use `--rsh-collect` when the filter needs the whole collection.

Related: [Pagination](/docs/guides/pagination/).
