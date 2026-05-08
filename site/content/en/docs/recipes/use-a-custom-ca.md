---
title: Use a Custom CA
linkTitle: Custom CA
weight: 76
description: Trust a private certificate authority for one request.
---

Private services often use certificates signed by an internal certificate
authority. Restish uses the normal system trust store by default; add
`--rsh-ca-cert` when one request should also trust your organization's CA file.

```bash
restish --rsh-ca-cert ./corp-ca.pem https://service.internal.test/items
```

Prerequisite: `corp-ca.pem` is the PEM-encoded CA certificate that signed the
server certificate.

Inspect the chain first:

```bash
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

The `cert` command helps confirm that the server presents the certificate chain
you expect before you debug application-level behavior. For repeated internal
requests, store TLS settings in a profile as described in [TLS](/docs/guides/tls/).

Related: [TLS](/docs/guides/tls/), [Commands](/docs/reference/commands/).
