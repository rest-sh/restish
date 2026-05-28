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
restish config trust
restish api list
```

If a repository has `.restish.json`, Restish discovers it from the current
directory or a parent but does not use it until you trust it. Trust is stored in
your user config state and includes the file's content hash, so changed project
config has to be trusted again.

Trusted project config layers project `apis` and `theme` over your global
config. Project APIs override global APIs with the same name, but unrelated
global APIs remain available. Normal config-writing commands still write your
global config and refuse to mutate project APIs; edit `.restish.json` directly
or pass `--rsh-config .restish.json` when you intentionally want that file to be
the complete config source for the command.

Committed `.restish.json` files should contain shared setup, not inline secrets.
Use non-secret values such as OAuth `client_id`, `audience`, scopes, and endpoint
URLs directly; use `env:NAME` references or omit values for API key values,
bearer tokens, passwords, and OAuth client secrets.

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
