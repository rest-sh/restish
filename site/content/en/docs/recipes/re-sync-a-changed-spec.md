---
title: Re-sync a Changed API Spec
linkTitle: Re-sync Spec
weight: 80
description: Refresh cached OpenAPI commands after the server spec changes.
---

```bash
restish api sync example
restish api show example
restish example --help
```

Use this after new operations, renamed operation IDs, changed tags, or updated
Restish `x-cli-*` extensions are published.

If commands still do not appear, confirm `spec_url` and inspect the API setup.

Related: [API Setup and Discovery](/docs/guides/api-setup-and-discovery/), [API Management](/docs/reference/api-management/).
