---
title: API Management
linkTitle: API Management
weight: 13
description: Reference for registering APIs, syncing specs, editing config, and inspecting API state.
---

`restish api` manages configured APIs and generated command sources.

## Connect

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
```

Discovers a spec, builds initial config, prompts for setup where needed, and
saves the API.

When a spec omits `x-cli-config`, Restish can derive initial auth setup from
OpenAPI security requirements. Declared but unused `components.securitySchemes`
do not create prompts or credential bindings until an operation actually
references them.

API names become command groups. Names may contain Unicode letters, Unicode
numbers, combining marks, `-`, and `_`, and must start with a letter or number.
They cannot collide with built-in commands such as `api`, `get`, or `post`, and
they cannot contain whitespace, URL/path delimiters, or shell punctuation.

Use `--no-discover` when you only want to save a local base URL without network
spec discovery:

```bash
restish api connect example api.rest.sh --no-discover
```

## Explicit Spec

```bash
restish api connect example api.rest.sh --spec https://api.rest.sh/openapi.json
```

Uses the provided OpenAPI file or URL instead of discovery. The explicit source
must be a supported OpenAPI document; unlike best-effort discovery, `--spec`
fails when the target is readable but is not an API spec. Rerun `api connect`
to refresh generated/default material; pass `--replace` when you want the rerun
to replace generated profile defaults instead of preserving local profile
edits. After writing, `api connect` prints the absolute config file path it
touched.

## Sync

```bash
restish api sync example
```

Forces a spec refresh after the API publishes changes.

## List And Inspect

```bash
restish api list
restish api list -o json
restish api inspect example
```

## Set And Config Edit

```bash
restish api set example 'command_layout: tags'
restish api set example 'operation_base: /v1'
restish api set example \
  'profiles.demo.auth: {type: bearer, params: {token: env:EXAMPLE_TOKEN}}'
restish config edit
```

`config edit` preserves comments where possible and prints the absolute config
file path after a successful write.

`api set` uses shorthand patch syntax rooted at `apis.<name>`. This command:

```bash
restish api set example 'profiles.demo.headers[]: "X-Debug: true"'
```

is equivalent to patching `apis.example.profiles.demo.headers[]` through
`config set`. Use `config set` when you need to patch outside one API.

## Remove

```bash
restish api remove example
```

Removes a configured API. It does not delete remote resources.

## Logout

```bash
restish api auth logout example
restish api auth logout example --all-profiles
restish api auth logout --auth-profile shared
```

Deletes cached OAuth/auth tokens so the next request performs a fresh auth
flow. This is separate from `cache clear`, which only deletes HTTP response
cache entries.

## Auth

```bash
restish api auth list example
restish api auth list example -o json
restish api auth add example PartnerKey
restish api auth remove example PartnerKey
restish api auth inspect example
restish api auth inspect example --rsh-credential basicAuth --redact
restish api auth header example Authorization basicAuth
```

`api auth` manages profile credential bindings for generated OpenAPI
operations. `inspect` replaces the old top-level auth helper and
prints every configured credential by default. It also works for
non-Authorization credentials such as API-key headers.

## Related Pages

- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [Config](../config/)
- [Commands](../commands/)
