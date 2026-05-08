---
title: TLS
linkTitle: TLS
weight: 95
description: Configure custom trust, mTLS certificates, TLS signer plugins, and certificate inspection.
---

Restish verifies TLS certificates by default. Custom CA and mTLS examples
require your own certificate infrastructure, so this page uses private hostnames
for those parts.

## Inspect A Server Certificate

```bash
restish cert api.rest.sh
restish cert --warn-days 14 api.rest.sh
```

## Trust A Custom CA

Use this when your organization uses a private CA:

```bash
restish --rsh-ca-cert ./corp-ca.pem https://service.internal.test/items
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

Prerequisite: `corp-ca.pem` must contain the PEM-encoded CA certificate that
signed the server certificate.

## Mutual TLS With Files

```bash
restish \
  --rsh-client-cert ./client.pem \
  --rsh-client-key ./client-key.pem \
  https://mtls.internal.test/items
```

Keep private keys out of shared repos and shell history.

## TLS Signer Plugins

Use a TLS signer plugin when the private key cannot leave hardware or another
security boundary:

```bash
restish \
  --rsh-tls-signer pkcs11 \
  --rsh-tls-signer-param module=/usr/local/lib/opensc-pkcs11.so \
  https://mtls.internal.test/items
```

## Minimum TLS Version

```bash
restish --rsh-tls-min-version TLS1.2 api.rest.sh
restish --rsh-tls-min-version TLS1.3 api.rest.sh
```

## Temporary Insecure Mode

```bash
restish --rsh-insecure https://service.internal.test/items
```

Use this only for short debugging sessions. Prefer `--rsh-ca-cert` for durable
configuration.

## Related Pages

- [Commands](/docs/reference/commands/)
- [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/)
- [Use a Custom CA](/docs/recipes/use-a-custom-ca/)
- [Use mTLS With a TLS Signer](/docs/recipes/use-mtls-with-a-tls-signer/)
