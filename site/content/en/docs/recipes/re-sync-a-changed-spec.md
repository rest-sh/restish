---
title: Re-sync a Changed API Spec
linkTitle: Re-sync Spec
weight: 73
description: Force Restish to refresh the cached API description when the upstream spec has changed.
---

If the API description changed and your generated commands are stale:

```bash
restish api sync myapi
```

Follow with:

```bash
restish myapi --help
```

to confirm that the generated command surface reflects the new spec.
