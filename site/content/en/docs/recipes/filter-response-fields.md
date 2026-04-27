---
title: Filter Response Fields
linkTitle: Filter Response Fields
weight: 30
description: Extract headers, links, or nested body fields from a response.
---

```bash
restish https://api.rest.sh/example -f body.basics.profiles
restish https://api.rest.sh/ -f headers.Content-Type -r
restish https://api.rest.sh/images -f links.next -r
```

Use jq for richer transforms:

```bash
restish https://api.rest.sh/images -f '.body[] | select(.format == "jpeg") | .name' -r
```

Related: [Filtering](/docs/guides/filtering/), [Query Syntax](/docs/reference/query-syntax/).
