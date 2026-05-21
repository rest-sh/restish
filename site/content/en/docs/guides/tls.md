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

`cert` is TLS-only. Bare hosts default to `https://`; `http://` targets are
rejected before any network connection.

## Trust A Custom CA

Use this when your organization uses a private CA:

```bash
restish --rsh-ca-cert ./corp-ca.pem https://service.internal.test/items
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

Prerequisite: `corp-ca.pem` must contain the PEM-encoded CA certificate that
signed the server certificate.
If the platform trust store cannot be loaded, Restish fails closed instead of
continuing with only the custom CA file.

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

Use TLS signer flags instead of `--rsh-client-cert` and `--rsh-client-key`.
Restish rejects a request that combines a signer with client certificate/key
files.

## Minimum TLS Version

Restish defaults to TLS 1.2. Pass `--rsh-tls-min-version TLS1.3` to require TLS 1.3.

```bash
restish --rsh-tls-min-version TLS1.2 api.rest.sh
restish --rsh-tls-min-version TLS1.3 api.rest.sh
```

## Temporary Insecure Mode

```bash
restish --rsh-insecure https://service.internal.test/items
```

Use this only for short debugging sessions. Prefer `--rsh-ca-cert` for durable
configuration. Restish treats `--rsh-insecure` as an explicit operator choice:
it does not prompt again, but verbose diagnostics make insecure verification
visible when troubleshooting.

## Related Pages

- [Utility Commands](/docs/reference/utility-commands/)
- [Global Flags](/docs/reference/global-flags/)
- [TLS Signer Plugins](/docs/plugins/tls-signer-plugins/)
- [Use a Custom CA](/docs/recipes/use-a-custom-ca/)
- [Use mTLS With a TLS Signer](/docs/recipes/use-mtls-with-a-tls-signer/)
