---
title: Post JSON From a File
linkTitle: Post JSON From a File
weight: 20
description: Send a JSON request body from stdin.
---

```bash
cat payload.json | restish post https://api.rest.sh/post
```

Example `payload.json`:

```json
{"name":"Alice","enabled":true}
```

The `/post` fixture echoes the parsed body so you can verify what was sent.

Variant with explicit content type:

```bash
cat payload.json | restish post -c json https://api.rest.sh/post
```

Related: [Input and Shorthand](/docs/guides/input/).
