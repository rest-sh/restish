---
title: Cert Command
linkTitle: Cert
weight: 40
description: Inspect a server TLS certificate chain.
---

Use `cert` before changing request TLS flags. It connects to the server,
inspects the certificate chain, and reports the details an operator usually
needs: issuer, subject, validity dates, and expiration warnings.

## Examples

```bash
restish cert https://api.rest.sh
restish cert --warn-days 14 https://api.rest.sh
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

The first command is a normal public certificate check. `--warn-days` changes
how soon Restish should warn about expiration. `--rsh-ca-cert` lets you inspect
a private service using the same custom CA file you would use for a request.

## Notes

Certificate diagnostics are written for operators, not for machine parsing. If
the chain does not validate, fix trust or server configuration before debugging
auth, profiles, or application responses. The [TLS guide](/docs/guides/tls/)
covers custom CAs, client certificates, and insecure debugging flags.

## Related Pages

- [Commands](/docs/reference/commands/)
- [TLS](/docs/guides/tls/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
