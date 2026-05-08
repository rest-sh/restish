---
title: Upload a File With Multipart
linkTitle: Multipart Upload
weight: 22
description: Send multipart/form-data with normal fields and a file.
---

Multipart requests are used for uploads that combine normal form fields with
file parts. `-c multipart` chooses the request encoding, and `@README.md` tells
Restish to send the file contents rather than the literal string.

```bash
restish post -c multipart api.rest.sh/uploads description: docs, file: @README.md
```

The response echoes multipart fields. If a client sends real file parts,
`/uploads` also reports file metadata. For plain URL-encoded forms, use
`-c form` instead; both encodings are explained in [Input and Shorthand](/docs/guides/input/).

Related: [Input and Shorthand](/docs/guides/input/), [Content Types](/docs/reference/content-types/).
