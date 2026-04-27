---
title: Use External-Tool Auth
linkTitle: External-Tool Auth
weight: 74
description: Delegate request auth to a local helper executable.
---

External-tool auth is for organizations that already have a credential helper,
request signer, SSO command, or token refresh program. Restish asks the helper
to prepare auth for the request instead of trying to own those credentials
itself.

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

Use this when another program owns credentials, signing, or token refresh. Keep
the helper small and auditable, because it runs locally with the same access as
your shell. The [Authentication guide](/docs/guides/authentication/) explains
where external tools fit alongside built-in auth types.

Related: [Authentication](/docs/guides/authentication/), [Security Design](/docs/contributing/design-records/).
