---
title: Force JSON Output to a File
linkTitle: Force JSON File
weight: 44
description: Write one JSON document to a file.
---

Redirected structured output defaults to JSON, but this recipe is intentionally
explicit because scripts often need a stable file shape. `--rsh-collect` matters
when pagination is involved: it asks Restish to build one logical document
before writing the file.

```bash
restish https://api.rest.sh/images --rsh-collect -o json > images.json
```

Open the file with `jq`, an editor, or another tool that expects one complete
JSON value. For line-oriented pipelines, prefer `-o ndjson` instead; the
[Output guide](/docs/guides/output/) explains document versus record formats.

Related: [Output](/docs/guides/output/), [Output Defaults](/docs/reference/output-defaults/).
