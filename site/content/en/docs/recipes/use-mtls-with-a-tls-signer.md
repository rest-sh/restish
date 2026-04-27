---
title: Use mTLS With a TLS Signer
linkTitle: mTLS TLS Signer
weight: 78
description: Use a TLS signer plugin when the client private key must stay outside Restish.
---

mTLS proves the client identity during the TLS handshake. A TLS signer plugin is
useful when the private key lives in hardware, an OS keychain, or another system
that should not hand raw key material to Restish.

```bash
restish \
  --rsh-tls-signer pkcs11 \
  --rsh-tls-signer-param module=/usr/local/lib/opensc-pkcs11.so \
  https://mtls.internal.test/items
```

Prerequisites: a configured signer plugin, a usable client certificate, and an
API that requires mTLS.

Use command-line flags for one-off debugging. For normal use, put signer
settings in a profile so generated commands stay readable. The operator flow is
covered in [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/).

Related: [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/), [TLS](/docs/guides/tls/).
