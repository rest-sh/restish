---
title: TLS
linkTitle: TLS
weight: 30
description: Configure custom trust, client certificates, and advanced TLS behavior in Restish.
---

Restish supports standard HTTPS verification, custom CA trust, mutual TLS, and
advanced client-certificate workflows.

## Secure By Default

Ordinary HTTPS verification is enabled by default. Most users should not need
to set any TLS flags for public APIs with standard certificates.

## Trust A Custom CA

For internal PKI or private certificate authorities, point Restish at a PEM
bundle:

```bash
restish --rsh-ca-cert ./corp-ca.pem https://api.example.com/items
```

This keeps verification enabled while extending the trust store for that
request.

## Mutual TLS With Certificate Files

For file-based mTLS, provide both the client certificate and its private key:

```bash
restish get \
  --rsh-client-cert ./client.pem \
  --rsh-client-key ./client.key \
  --rsh-ca-cert ./ca.pem \
  https://api.example.com/items
```

These options also fit naturally into profile-based workflows when you need
repeatable configuration.

Use file-based mTLS when you have ordinary PEM material on disk. Use a TLS
signer plugin when the private key must stay outside the Restish process.

## TLS Signer Plugins

When the private key must stay outside the Restish process, use a TLS signer
plugin instead of a local key file.

Relevant flags:

- `--rsh-tls-signer`
- `--rsh-tls-signer-param key=value`

This is the advanced path for hardware-backed keys, HSMs, or external signing
systems.

## Minimum TLS Version

If you need to restrict protocol negotiation:

```bash
restish --rsh-tls-min-version TLS1.2 https://api.example.com/items
restish --rsh-tls-min-version TLS1.3 https://api.example.com/items
```

## Temporary Insecure Mode

`--rsh-insecure` disables certificate verification:

```bash
restish --rsh-insecure https://api.example.com/items
```

Use this only for temporary debugging. Restish warns when verification is
disabled because the connection is no longer meaningfully authenticated.

## Inspect Server Certificates

Use the `cert` command to inspect the presented certificate chain:

```bash
restish cert https://api.example.com
restish cert --rsh-ca-cert ./corp-ca.pem https://api.example.com
restish cert --warn-days 14 https://api.example.com
```

This is useful for checking issuers, names, expiry windows, and the exact trust
context Restish itself would use.

## Common Failure Modes

- server signed by a private CA you have not trusted yet
- wrong client certificate or private key
- using file-based mTLS when the key actually lives in hardware
- minimum TLS version set higher than the server supports

## Related Guides

- [Authentication](../authentication/)
- [TLS Signer Plugins](../plugins/tls-signer-plugins/)

Source material:

- [Design Records](/docs/contributing/design-records/)
