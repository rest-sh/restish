---
title: Show Links for One Relation
linkTitle: One Link Relation
weight: 77
description: Print only one normalized link relation from a response.
---

Use the `links` command with a relation name:

```bash
restish links https://api.rest.sh/images next
```

Or filter the normalized response directly:

```bash
restish https://api.rest.sh/images -f links.next -r
```
