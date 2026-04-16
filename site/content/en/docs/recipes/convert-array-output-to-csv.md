---
title: Convert Array Output to CSV
linkTitle: Convert to CSV
weight: 75
description: Use the csv formatter plugin when you want array-shaped results as CSV.
---

If the `restish-csv` formatter plugin is installed, array-shaped results can be
rendered as CSV:

```bash
restish https://api.rest.sh/images -o csv
```

This is most useful for arrays of similarly shaped objects.

Confirm plugin discovery first if needed:

```bash
restish plugin list
```
