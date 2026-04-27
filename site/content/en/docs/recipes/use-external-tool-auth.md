---
title: Use External-Tool Auth
linkTitle: External-Tool Auth
weight: 74
description: Delegate request auth to a local helper executable.
---

Config shape:

```jsonc
{
  "apis": {
    "vendor": {
      "base_url": "https://api.vendor.test",
      "profiles": {
        "default": {
          "auth": {
            "type": "external-tool",
            "params": {
              "command": ["./scripts/sign-request"]
            }
          }
        }
      }
    }
  }
}
```

Restish approves external tools by command hash. If the helper changes, you
must approve it again.

Use this when another program owns credentials, signing, or token refresh.

Related: [Authentication](/docs/guides/authentication/), [Security Design](/docs/contributing/design-records/).
