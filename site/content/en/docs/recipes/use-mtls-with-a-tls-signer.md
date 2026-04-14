---
title: Use mTLS With a TLS Signer
linkTitle: Use mTLS With a TLS Signer
weight: 70
description: Use a TLS signer plugin when the client private key must stay outside the Restish process.
---

When your client certificate key lives in a hardware token, HSM, or external
signing system, use a TLS signer plugin instead of `--rsh-client-key`.

Example profile config:

```json
{
  "apis": {
    "corp": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "tls_signer": "restish-pkcs11",
          "tls_signer_params": {
            "module": "/usr/local/lib/opensc-pkcs11.so",
            "token_label": "YubiKey"
          }
        }
      }
    }
  }
}
```

Then make requests normally:

```bash
restish corp/items
```
