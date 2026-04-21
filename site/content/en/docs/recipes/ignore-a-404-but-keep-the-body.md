---
title: Ignore a 404 but Keep the Body
linkTitle: Ignore a 404
weight: 45
description: Keep the response body even when the HTTP status would normally produce a failing exit code.
---

Use `--rsh-ignore-status-code` when you want the response body but do not want
the command to fail because of the HTTP status:

```bash
restish https://api.rest.sh/missing --rsh-ignore-status-code
```

This is especially useful when:

- the server returns structured error JSON you want to inspect
- you are debugging an API integration
- you are capturing the body inside a script
