---
title: Re-sync a Changed API Spec
linkTitle: Re-sync Spec
weight: 80
description: Refresh cached OpenAPI commands after the server spec changes.
---

Generated commands come from the API description Restish cached when you
configured the API. If the server publishes a new operation or changes command
metadata, sync the API so local help, completion, and generated commands match
the current spec.

```bash
restish api sync example
restish api inspect example
restish example --help
```

Use this after new operations, renamed operation IDs, changed tags, or updated
Restish `x-cli-*` extensions are published.

If commands still do not appear, confirm `spec_url` and inspect the API setup.
When `spec_url` is configured, sync uses that exact URL as the authoritative
spec source. Update it before syncing if the API moved its OpenAPI document.
The [OpenAPI integration guide](/docs/guides/openapi-cli-integration/) explains
how operation IDs, tags, and `x-cli-*` extensions become command names.

Related: [API Setup and Discovery](/docs/guides/api-setup-and-discovery/), [API Management](/docs/reference/api-management/).
