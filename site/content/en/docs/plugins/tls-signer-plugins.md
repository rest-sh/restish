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

Prerequisites: the signer plugin is installed, the signer can access the client
certificate and signing key material it needs, and the target API requires
mTLS.

## Profile Shape

```jsonc
{
  "apis": {
    "secure": {
      "base_url": "https://mtls.internal.test",
      "profiles": {
        "default": {
          "ca_cert": "/etc/ssl/internal-ca.pem",
          "tls_signer": "pkcs11",
          "tls_signer_params": {
            "module": "/usr/local/lib/opensc-pkcs11.so",
            "token_label": "Restish",
            "pin_env": "PKCS11_PIN"
          }
        }
      }
    }
  }
}
```

Profile-level TLS settings override API-level defaults for that profile. Use
either `client_cert`/`client_key` files or `tls_signer` for one request; Restish
rejects configs that combine both client-certificate files and a TLS signer.

CLI flags such as `--rsh-client-cert`, `--rsh-client-key`, `--rsh-ca-cert`, and
`--rsh-tls-signer` override both config layers for the current command.

For the bundled PKCS#11 signer, set `module` or `path`, at most one token
selector (`token_label`/`label`, `token_serial`/`serial`, or `slot`), and
either `pin`, `pin_env`, `PKCS11_PIN`, or `login_not_supported: true`. If you
omit the token selector, the plugin auto-selects the token only when exactly one
token is present.

## Troubleshooting

```bash
restish plugin list
restish cert https://mtls.internal.test
restish -vv https://mtls.internal.test/items
```

## Related Pages

- [TLS](/docs/guides/tls/)
- [Use mTLS With a TLS Signer](/docs/recipes/use-mtls-with-a-tls-signer/)
- [Plugin Quickstart](../quickstart/)
- [Plugin Manifest](/docs/reference/plugin-manifest/)
- [Plugin Messages](/docs/reference/plugin-messages/)
