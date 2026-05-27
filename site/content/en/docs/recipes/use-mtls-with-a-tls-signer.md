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

Prerequisites: a configured signer plugin that can access the client
certificate and signing key material, and an API that requires mTLS. Replace
`mtls.internal.test` and signer parameters with your environment's values.

Use command-line flags for one-off debugging. For normal use, put signer
settings in a profile so generated commands stay readable. The operator flow is
covered in [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/).

Do not combine `--rsh-tls-signer` with `--rsh-client-cert` or
`--rsh-client-key`; the signer path supplies the client certificate used for the
handshake.

Related: [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/), [TLS](/docs/guides/tls/), [Global Flags](/docs/reference/global-flags/).
