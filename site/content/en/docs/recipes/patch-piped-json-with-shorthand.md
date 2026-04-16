---
title: Patch Piped JSON With Shorthand
linkTitle: Patch Piped JSON
weight: 26
description: Treat piped structured input as the base document and apply shorthand overrides on top.
---

When stdin already contains structured JSON, you can patch it with shorthand
arguments:

```bash
echo '{"name":"Alice","role":"user"}' | \
  restish post https://api.rest.sh role: admin
```

Equivalent body:

```json
{
  "name": "Alice",
  "role": "admin"
}
```
