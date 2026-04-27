---
title: Use a Custom CA
linkTitle: Custom CA
weight: 76
description: Trust a private certificate authority for one request.
---

```bash
restish --rsh-ca-cert ./corp-ca.pem https://service.internal.test/items
```

Prerequisite: `corp-ca.pem` is the PEM-encoded CA certificate that signed the
server certificate.

Inspect the chain first:

```bash
restish cert --rsh-ca-cert ./corp-ca.pem https://service.internal.test
```

Related: [TLS](/docs/guides/tls/), [Cert Command](/docs/reference/cert-command/).
