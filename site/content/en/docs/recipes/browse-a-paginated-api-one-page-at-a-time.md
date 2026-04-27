---
title: Browse a Paginated API One Page at a Time
linkTitle: One Page at a Time
weight: 56
description: Disable automatic pagination to inspect the first page and its next link.
---

```bash
restish https://api.rest.sh/images --rsh-no-paginate
```

Show only the next link:

```bash
restish https://api.rest.sh/images --rsh-no-paginate -f links.next -r
```

Use this before tuning `--rsh-max-pages`, `--rsh-max-items`, or collect mode.

Related: [Pagination](/docs/guides/pagination/).
