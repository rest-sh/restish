---
title: Cert Command
linkTitle: Cert
weight: 40
description: Inspect a server TLS certificate chain.
---

Inspect a server TLS certificate chain.

## Examples

```bash
restish cert https://api.rest.sh
restish cert --warn-days 14 https://api.rest.sh
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

## Notes

Use this before changing TLS flags. Certificate diagnostics are written for operators, not for machine parsing.

## Related Pages

- [Commands](/docs/reference/commands/)
- [Global Flags](/docs/reference/global-flags/)
- [Troubleshooting](/docs/guides/troubleshooting/)
