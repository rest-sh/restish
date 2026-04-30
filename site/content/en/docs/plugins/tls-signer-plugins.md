---
title: TLS Signer Plugins
linkTitle: TLS Signer Plugins
weight: 50
description: Use or author plugins that sign mTLS handshakes without exposing private keys to Restish.
---

TLS signer plugins are for mTLS environments where the client private key lives
in hardware, a secure service, or another process.

## Operator Example

```bash
restish \
  --rsh-tls-signer pkcs11 \
  --rsh-tls-signer-param module=/usr/local/lib/opensc-pkcs11.so \
  https://mtls.internal.test/items
```

Prerequisites: the signer plugin is installed, the client certificate is
available, and the target API requires mTLS.

## Profile Shape

```jsonc
{
  "apis": {
    "secure": {
      "base_url": "https://mtls.internal.test",
      "profiles": {
        "default": {
          "ca_cert": "/etc/ssl/internal-ca.pem",
          "client_cert": "/etc/ssl/client.pem",
          "client_key": "/etc/ssl/client.key",
          "tls_signer": "pkcs11",
          "tls_signer_params": {
            "module": "/usr/local/lib/opensc-pkcs11.so"
          }
        }
      }
    }
  }
}
```

Profile-level TLS settings override API-level defaults for that profile. CLI
flags such as `--rsh-client-cert`, `--rsh-client-key`, `--rsh-ca-cert`, and
`--rsh-tls-signer` override both config layers for the current command.

## Troubleshooting

```bash
restish plugin list
restish cert https://mtls.internal.test
restish -vv https://mtls.internal.test/items
```

## Related Pages

- [TLS](/docs/guides/tls/)
- [Use mTLS With a TLS Signer](/docs/recipes/use-mtls-with-a-tls-signer/)
- [Plugin Manifest](/docs/reference/plugin-manifest/)
