---
title: Count Items Across All Pages
linkTitle: Count Across Pages
weight: 55
description: Collect a paginated response before counting items.
---

Counting is a whole-collection operation. If Restish streams paginated items as
they arrive, a filter can start earlier but it does not necessarily see the
entire logical result at once. `--rsh-collect` tells Restish to gather the
paginated response before applying the filter.

```bash
restish https://api.rest.sh/images --rsh-collect -f '.body | length'
```

Example output:

```text
5
```

Use this pattern for counts, sorts, unique values, or summaries. For
item-by-item processing, skip collect mode and use a record format such as
`ndjson`; the [Pagination guide](/docs/guides/pagination/) explains the
tradeoff.

Related: [Pagination](/docs/guides/pagination/).
