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

Use placeholder hosts here because certificate inspection is usually about your
real target service and trust context rather than the public example API.

## Important Flags

- `--warn-days`: exit non-zero if the leaf certificate expires within N days
- `--rsh-ca-cert`: trust an extra PEM CA bundle during inspection

## What The Output Tells You

`restish cert` is the fastest way to inspect:

- subject and issuer information
- SAN names
- expiry windows
- whether your additional CA bundle changes trust outcomes

That makes it useful both for manual debugging and for simple certificate
freshness checks in automation.

## Related Pages

- [TLS](/docs/guides/tls/)
- [API Management](../api-management/)
