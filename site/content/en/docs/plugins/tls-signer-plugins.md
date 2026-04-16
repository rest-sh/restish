---
title: TLS Signer Plugins
linkTitle: TLS Signer Plugins
weight: 40
description: Use external signing plugins for hardware-backed or non-exportable client keys.
---

TLS signer plugins support advanced mutual TLS workflows where the private key
must remain outside the Restish process.

## When You Need One

Use a TLS signer plugin when:

- the private key lives in a hardware token
- the key is managed by PKCS#11
- the signing operation must happen in another process

If you have an ordinary PEM certificate and key file, use the built-in
`--rsh-client-cert` and `--rsh-client-key` flags instead.

## How They Fit Into Restish

TLS signer selection is part of the same request configuration model as the
rest of Restish:

- set `tls_signer` and `tls_signer_params` in a profile
- or use `--rsh-tls-signer` and `--rsh-tls-signer-param` for one request

That means you do not need a separate mTLS config system just because the key
material is external.

## PKCS#11 Example

The first-party `restish-pkcs11` plugin is the main example of this model.

Profile-based configuration:

```jsonc
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

One-off command-line override:

```bash
restish \
  --rsh-tls-signer restish-pkcs11 \
  --rsh-tls-signer-param module=/usr/local/lib/opensc-pkcs11.so \
  --rsh-tls-signer-param token_label=YubiKey \
  https://api.example.com/items
```

## Troubleshooting

If a TLS signer setup fails, check these first:

- the plugin binary is installed and discoverable
- the signer name matches the plugin name Restish sees
- the module path is correct
- the token label or selector matches the real hardware token
- the certificate actually chains to what the server expects

Useful companion commands:

```bash
restish plugin list
restish cert https://api.example.com
restish -vv https://api.example.com/items
```

Primary sources:

- [`docs/design/021-tls-signer-plugins.md`](/docs/contributing/design-records/)
- [`docs/design/022-restish-pkcs11-plugin.md`](/docs/contributing/design-records/)
