---
title: API Setup and Discovery
linkTitle: API Setup and Discovery
weight: 15
description: Register APIs, discover OpenAPI specs, manage generated command sources, and sync changes.
aliases:
  - /docs/recipes/re-sync-a-changed-spec/
---

Restish can call any URL, but registering an API gives you generated commands,
profiles, auth setup, spec caching, and completion.

## Configure By Discovery

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
restish example --help
```

Restish looks for an OpenAPI description through well-known locations and link
relations, then caches the spec.

Discovery is intentionally conservative. Restish trusts same-origin spec links
by default; use `--allow-cross-origin-spec` only when you expect the API to
advertise its OpenAPI document from another host and you trust that host.
Cross-origin discovery still rejects private, loopback, link-local, multicast,
and unspecified follow targets unless the original API is already private or
local. Use `--spec` when you need to name a private spec URL directly.

If the spec contains behavior-changing `x-cli-*` extensions, `api connect`
prints a compact summary. Run `restish doctor api <name>` after connecting when
you want to see the exact operations or parameters affected.

## Configure With An Explicit Spec

```bash
restish api connect example api.rest.sh --spec https://api.rest.sh/openapi.json
restish api sync example
```

Use this when discovery is unavailable or the API publishes its spec at a
non-standard path. The explicit source must be a supported OpenAPI document;
Restish fails instead of saving the API when the file or URL is readable but is
not actually an API spec. Once `spec_url` is configured, Restish treats it as
the authoritative source for that API. `api sync` fetches that URL directly
instead of falling back to well-known discovery probes.

## Inspect And Edit Config

```bash
restish api list
restish api inspect example
restish api set example 'command_layout: tags'
restish config edit
```

`api set` accepts shorthand-style path updates. `config edit` is better for larger
changes and preserves comments where possible.

When you reconnect an existing API with `api connect`, Restish preserves
existing profiles by default because they may contain credentials or auth
references. API-level fields are refreshed from the new connect run. Add
`--replace` only when you want OpenAPI or `x-cli-config` profile defaults to
replace existing profiles.

## Operation Base

Use `operation_base` when operations live under a path prefix:

```bash
restish api set example 'operation_base: /v1'
```

Keep it path-only. Use `base_url` for scheme and host.

## Project Config Files

```bash
restish --rsh-config ./restish.json api connect example api.rest.sh
restish --rsh-config ./restish.json api list
```

An explicit config file is not merged with the global config. Missing explicit
files fail clearly instead of silently falling back. Restish does not
automatically discover project config from the current directory; pass
`--rsh-config` or set `RSH_CONFIG` when a repository should use its own config.

## Sync After Spec Changes

```bash
restish api sync example
restish example --help
```

Sync when the API publishes new operations, changes operation names, or updates
OpenAPI extensions that shape the CLI. Sync can also save spec-derived API
metadata that changed after registration, such as a Link-discovered `spec_url`
or newly discovered operation-server origins. It does not overwrite profiles or
apply new `x-cli-config` profile defaults, so local credentials remain intact.
When the refreshed spec contains behavior-changing `x-cli-*` extensions, sync
prints the same compact summary as connect.

Use `--yes` when an automated sync should accept safe metadata prompts such as
allowing a newly discovered cross-origin operation server:

```bash
restish api sync example --yes
```

If the API moves its OpenAPI document, update `spec_url` before syncing:

```bash
restish api set example 'spec_url: https://api.rest.sh/openapi.json'
restish api sync example
```

## Related Pages

- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [API Management](/docs/reference/api-management/)
- [Config Command](/docs/reference/config-command/)
- [OpenAPI Reference](/docs/reference/openapi-cli-integration/)
- [Troubleshooting](../troubleshooting/)
