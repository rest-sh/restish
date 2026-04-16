---
title: Force JSON Output to a File
linkTitle: Force JSON File
weight: 41
description: Force one JSON document to disk even when the endpoint paginates or the output is filtered.
---

Use `-o json` when you want to be explicit that the output on disk must be one
JSON document:

```bash
restish https://api.rest.sh/images -o json > images.json
restish https://api.rest.sh/images --rsh-collect -f '.body | map(.self)' -o json > paths.json
```
