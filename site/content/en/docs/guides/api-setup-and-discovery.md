---
title: API Setup and Discovery
linkTitle: API Setup and Discovery
weight: 15
description: Register APIs, discover OpenAPI specs, manage generated command sources, and sync changes.
---

Restish can call any URL, but registering an API gives you generated commands,
profiles, auth setup, spec caching, and completion.

## Configure By Discovery

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
restish example --help
```

Restish looks for an OpenAPI description through well-known locations and link
relations, then caches the spec.

## Configure With An Explicit Spec

```bash
restish api add example https://api.rest.sh spec_url: https://api.rest.sh/openapi.json
restish api sync example
```

Use this when discovery is unavailable or the API publishes its spec at a
non-standard path.

## Inspect And Edit Config

```bash
restish api list
restish api show example
restish api set example command_layout: tags
restish api edit
```

`api set` accepts shorthand-style path updates. `api edit` is better for larger
changes and preserves comments where possible.

## Operation Base

Use `operation_base` when operations live under a path prefix:

```bash
restish api set example operation_base: /v1
```

Keep it path-only. Use `base_url` for scheme and host.

## Project Config Files

```bash
restish --rsh-config ./restish.json api add example https://api.rest.sh
restish --rsh-config ./restish.json api list
```

An explicit config file is not merged with the global config. Missing explicit
files fail clearly instead of silently falling back.

## Sync After Spec Changes

```bash
restish api sync example
restish example --help
```

Sync when the API publishes new operations, changes operation names, or updates
OpenAPI extensions that shape the CLI.

## Related Pages

- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [API Management](/docs/reference/api-management/)
- [OpenAPI and CLI Integration](../openapi-cli-integration/)
- [Troubleshooting](../troubleshooting/)
