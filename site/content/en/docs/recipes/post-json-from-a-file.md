---
title: Post JSON From a File
linkTitle: Post JSON From a File
weight: 20
description: Send a JSON request body from stdin.
---

Use a file when the request body is too large or too important to type as
shorthand on the command line. Restish reads stdin, decodes the structured
document, and sends it as the request body.

```bash
cat payload.json | restish post api.rest.sh/post
```

Example `payload.json`:

```json
{"name":"Alice","enabled":true}
```

The `/post` fixture echoes the parsed body so you can verify what was sent.

Variant with explicit content type:

```bash
cat payload.json | restish post -c json api.rest.sh/post
```

The explicit `-c json` form is useful when stdin comes from a source without a
clear extension or when a script should make the request encoding obvious. For
small bodies, [shorthand](/docs/reference/shorthand/) is usually faster.

Related: [Input and Shorthand](/docs/guides/input/).
