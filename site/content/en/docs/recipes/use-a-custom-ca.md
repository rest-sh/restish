---
title: Use a Custom CA
linkTitle: Use a Custom CA
weight: 71
description: Trust a private or internal PEM CA bundle without disabling TLS verification.
---

Use `--rsh-ca-cert` when the server is signed by a private CA:

```bash
restish --rsh-ca-cert ./corp-ca.pem https://api.example.com/items
```

This keeps verification enabled while extending the trust store for that
request.
