---
title: Cert Command
linkTitle: Cert Command
weight: 15
description: Reference for restish cert, which inspects server TLS certificate chains.
---

`restish cert <uri>` inspects the server certificate chain using the same trust
context Restish would use for requests.

## Examples

```bash
restish cert https://api.example.com
restish cert --rsh-ca-cert ./corp-ca.pem https://api.example.com
restish cert --warn-days 14 https://api.example.com
```

## Important Flags

- `--warn-days`: exit non-zero if the leaf certificate expires within N days
- `--rsh-ca-cert`: trust an extra PEM CA bundle during inspection

## Related Pages

- [TLS](/docs/guides/tls/)
- [API Management](../api-management/)
