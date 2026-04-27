---
title: Use mTLS With a TLS Signer
linkTitle: mTLS TLS Signer
weight: 78
description: Use a TLS signer plugin when the client private key must stay outside Restish.
---

```bash
restish \
  --rsh-tls-signer pkcs11 \
  --rsh-tls-signer-param module=/usr/local/lib/opensc-pkcs11.so \
  https://mtls.internal.test/items
```

Prerequisites: a configured signer plugin, a usable client certificate, and an
API that requires mTLS.

Related: [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/), [TLS](/docs/guides/tls/).
