---
title: Use External-Tool Auth
linkTitle: External Tool Auth
weight: 74
description: Delegate request auth to a local helper script instead of storing secrets directly in Restish config.
---

Example config:

```jsonc
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "external-tool",
            "params": {
              "commandline": "./scripts/sign-request.sh",
              "omitbody": "true"
            }
          }
        }
      }
    }
  }
}
```

Then make requests normally:

```bash
restish myapi/items
```
