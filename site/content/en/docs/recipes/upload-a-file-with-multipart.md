---
title: Upload a File With Multipart
linkTitle: Multipart Upload
weight: 22
description: Send multipart/form-data with normal fields and a file.
---

```bash
restish post -c multipart https://api.rest.sh/uploads description: docs, file: @README.md
```

The response echoes multipart fields. If a client sends real file parts, `/uploads` also reports file metadata.

Related: [Input and Shorthand](/docs/guides/input/), [Content Types](/docs/reference/content-types/).
