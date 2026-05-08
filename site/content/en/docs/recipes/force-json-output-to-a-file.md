---
title: Force JSON Output to a File
linkTitle: Force JSON File
weight: 44
description: Write one JSON document to a file.
---

Redirected output saves response body bytes by default. Choose `-o json` when a
script needs Restish to render decoded structured data as one JSON document.
`--rsh-collect` matters when pagination is involved: it asks Restish to build
one logical document before writing the file.

```bash
restish https://api.rest.sh/images --rsh-collect -o json > images.json
```

Open the file with `jq`, an editor, or another tool that expects one complete
JSON value. For line-oriented pipelines, prefer `-o ndjson` instead; the
[Output guide](/docs/guides/output/) explains document versus record formats.

Related: [Output](/docs/guides/output/), [Output Defaults](/docs/reference/output-defaults/).
