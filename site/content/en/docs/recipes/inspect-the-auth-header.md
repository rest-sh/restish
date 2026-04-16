---
title: Inspect the Auth Header
linkTitle: Inspect Auth Header
weight: 72
description: See the Authorization header Restish would send without making the full request.
---

Use:

```bash
restish auth-header myapi
restish -p ci auth-header myapi
```

This is helpful when you need to confirm:

- which profile is active
- whether a token is cached
- which auth mechanism Restish resolved
