---
title: Force JSON Output to a File
linkTitle: Force JSON File
weight: 44
description: Write one JSON document to a file.
---

```bash
restish https://api.rest.sh/images --rsh-collect -o json > images.json
```

Use `--rsh-collect` when pagination should become one logical document before
JSON formatting.

Related: [Output](/docs/guides/output/), [Output Defaults](/docs/reference/output-defaults/).
