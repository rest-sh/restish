---
title: API Management
linkTitle: API Management
weight: 13
description: Reference for registering APIs, syncing specs, editing config, and inspecting API state.
---

`restish api` manages configured APIs and generated command sources.

## Generated Command Reference

<!-- BEGIN GENERATED: restish-docgen api-command -->
Generated from the current Cobra command tree.

### `restish api`

Manage registered API configurations

Manage APIs registered in the local Restish config.

Registered APIs turn OpenAPI descriptions into generated commands with shell completion, persistent profiles, and auth-aware requests. Use `api connect` to add an API, `api sync` after its OpenAPI document changes, and `api set` for local profile edits.

Usage:

```text
restish api
```

Examples:

```bash
  restish api connect demo https://api.example.com
  restish api list
  restish api set demo 'profiles.staging.base_url: https://staging.example.com'
```

Subcommands:

**`restish api auth`**: Manage API auth credentials

**`restish api connect`**: Connect Restish to an API and discover generated commands

**`restish api inspect`**: Print the config for a registered API as JSON

**`restish api list`**: List all configured APIs

**`restish api remove`**: Remove a configured API

**`restish api set`**: Patch API config using shorthand syntax

**`restish api sync`**: Force re-fetch of the cached OpenAPI spec for a named API


### `restish api connect`

Connect Restish to an API and discover generated commands

Connect Restish to an API, discover its OpenAPI description, and save a named API profile.

Use this when repeated work against an API deserves generated commands, shell completion, auth setup, and profile-aware defaults.

Common choices:

- Use `--spec` when discovery is blocked, the API does not advertise its spec, or you want to pin setup to a known OpenAPI URL or local file.
- Use `--allow-cross-origin-spec` only when you trust a `Link` header that points to an OpenAPI document on another host. Private, loopback, link-local, and unspecified IP literal spec targets are still rejected.
- Use `--no-discover` to save a base URL without fetching a spec.
- Use `--replace` when reconnecting should replace generated OpenAPI or `x-cli-config` profile defaults instead of preserving local profile edits.
- Use `--yes` only for safe connect prompts you have already decided to accept in automation.

Usage:

```text
restish api connect <name> <url> [setup-expression ...] [flags]
```

Examples:

```bash
  restish api connect demo https://api.example.com
  restish api connect demo https://api.example.com 'prompt.api_key: env:DEMO_API_KEY'
  restish api connect demo https://api.example.com --spec ./openapi.yaml
```

Flags:

**`--allow-cross-origin-spec`**

Type: `bool`; default: `false`

Allow Link-header spec discovery from another host; private and loopback IP literals are still rejected

**`--no-discover`**

Type: `bool`; default: `false`

Register the API locally without network spec discovery

**`--replace`**

Type: `bool`; default: `false`

Replace existing profiles with discovered OpenAPI/x-cli-config defaults

**`--spec`**

Type: `string`; default: none

OpenAPI spec URL or local file to use instead of discovery

**`--yes`**

Type: `bool`; default: `false`

Accept safe api connect prompts without asking



### `restish api sync`

Force re-fetch of the cached OpenAPI spec for a named API

Force re-fetch of the cached OpenAPI spec for a named API.

Use this after the API publishes new operations, updates parameter schemas, or changes `x-cli-config` defaults that should affect generated commands.

By default, sync follows the same-origin spec source already recorded for the API. Use `--allow-cross-origin-spec` only when you trust a `Link` header or saved spec source that points to an OpenAPI document on another host. Private, loopback, link-local, and unspecified IP literal spec targets are still rejected.

Usage:

```text
restish api sync <name> [flags]
```

Examples:

```bash
  restish api sync demo
  restish api sync demo --allow-cross-origin-spec
```

Flags:

**`--allow-cross-origin-spec`**

Type: `bool`; default: `false`

Allow Link-header spec discovery from another host for this sync run



### `restish api list`

List all configured APIs

List every API registered in the active Restish config.

Use `-o json` when scripts need stable fields such as API names, base URLs, and profile counts. Human output is a compact inventory for deciding what to inspect, sync, or remove next.

Usage:

```text
restish api list
```

Examples:

```bash
  restish api list
  restish api list -o json
```


### `restish api inspect`

Print the config for a registered API as JSON

Print the saved config for one registered API as JSON.

Use this when you need the exact merged API entry, including profiles, headers, query defaults, auth config, spec URLs, and generated-command settings. Sensitive values may still be present if they are stored directly in config.

Usage:

```text
restish api inspect <name>
```

Examples:

```bash
  restish api inspect demo
```


### `restish api set`

Patch API config using shorthand syntax

Patch one registered API using Restish shorthand syntax.

Use this for durable local overrides such as profile URLs, default headers, query parameters, auth settings, and server variables. Patches are applied to the saved config file; they do not update the remote API or the cached OpenAPI document.

Run `restish api inspect <name>` first when you want to confirm the current config shape.

Usage:

```text
restish api set <name> <patch> [patch...]
```

Examples:

```bash
  restish api set demo 'profiles.default.headers[]: X-Trace-Id: abc'
  restish api set demo 'base_url: https://staging.example.com'
  restish api set demo 'profiles.prod.auth.type: oauth-client-credentials'
```


### `restish api remove`

Remove a configured API

Remove a registered API from the local config.

This deletes the saved API definition and generated-command source for that name. It does not contact the remote API, delete server-side resources, or remove unrelated HTTP cache and token cache entries.

Usage:

```text
restish api remove <name>
```

Examples:

```bash
  restish api remove demo
```


### `restish api auth`

Manage API auth credentials

Manage auth material for a registered API profile.

Use these commands when a generated OpenAPI command reports missing auth, when you want to see which credentials satisfy secured operations, or when cached OAuth tokens need to be cleared.

Most commands honor `--rsh-profile` so you can inspect or update a non-default API profile.

Usage:

```text
restish api auth
```

Examples:

```bash
  restish api auth inspect demo
  restish api auth inspect demo --rsh-operation list-items
  restish api auth logout demo
```

Subcommands:

**`restish api auth add`**: Add an empty credential binding to an API profile

**`restish api auth header`**: Print one auth header value for an API profile

**`restish api auth inspect`**: Inspect the auth material applied for an API profile

**`restish api auth logout`**: Delete cached API auth tokens

**`restish api auth remove`**: Remove a credential binding from an API profile


### `restish api auth add`

Add an empty credential binding to an API profile

Add or initialize a credential binding for an API profile.

Use this after `api auth inspect` reports a missing credential ID. When cached OpenAPI auth metadata is available, Restish can prefill auth settings and prompt for the parameters needed by that credential.

Usage:

```text
restish api auth add <api> <credential-id>
```

Examples:

```bash
  restish api auth add demo PartnerKey
```


### `restish api auth remove`

Remove a credential binding from an API profile

Remove one credential binding from an API profile.

This edits local Restish config only. It does not revoke remote tokens or delete cached OAuth tokens; run `api auth logout` when cached tokens should be cleared too.

Usage:

```text
restish api auth remove <api> <credential-id>
```

Examples:

```bash
  restish api auth remove demo PartnerKey
```


### `restish api auth logout`

Delete cached API auth tokens

Delete cached API auth tokens.

Use this when credentials changed, an OAuth grant should be refreshed, or a shared auth profile should forget cached tokens.

- Pass an API name to clear the current `--rsh-profile` token cache entry.
- Add `--all-profiles` to clear every profile for that API.
- Use `--auth-profile` to clear a shared auth profile cache without naming an API.

Usage:

```text
restish api auth logout [api] [flags]
```

Examples:

```bash
  restish api auth logout demo
  restish api auth logout demo --all-profiles
  restish api auth logout --auth-profile shared-oauth
```

Flags:

**`--all-profiles`**

Type: `bool`; default: `false`

Delete cached auth tokens for every profile of the named API

**`--auth-profile`**

Type: `string`; default: none

Delete cached auth tokens for a shared auth profile instead of an API



### `restish api auth header`

Print one auth header value for an API profile

Print one auth header value that Restish would apply for an API profile.

Use this for debugging generated-command auth without sending a request. Pass `--rsh-operation` to inspect operation-specific security requirements, or `--rsh-credential` to inspect a named credential binding directly.

Usage:

```text
restish api auth header <api> <header> [credential-id] [flags]
```

Examples:

```bash
  restish api auth header demo Authorization
  restish api auth header demo X-API-Key PartnerKey
```

Flags:

**`--rsh-credential`**

Type: `string`; default: none

Credential ID to inspect instead of profile-level auth

**`--rsh-operation`**

Type: `string`; default: none

Operation ID or command name to inspect



### `restish api auth inspect`

Inspect the auth material applied for an API profile

Inspect auth readiness and material for an API profile.

By default this shows configured credentials, generated-operation coverage, and the auth values Restish would apply. Use `--rsh-operation` for operation-specific OpenAPI security requirements or `--rsh-credential` for one credential binding. Add `--redact` before sharing output so sensitive header, token, and credential values are masked.

Usage:

```text
restish api auth inspect <api> [flags]
```

Examples:

```bash
  restish api auth inspect demo
  restish api auth inspect demo --rsh-operation list-items --redact
```

Flags:

**`--redact`**

Type: `bool`; default: `false`

Redact sensitive auth values for shareable output

**`--rsh-credential`**

Type: `string`; default: none

Credential ID to inspect instead of profile-level auth

**`--rsh-operation`**

Type: `string`; default: none

Operation ID or command name to inspect
<!-- END GENERATED -->

## Workflow Examples

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
restish api connect example api.rest.sh --spec https://api.rest.sh/openapi.json
restish api sync example
restish api list -o json
restish api inspect example
```

The generated command reference above covers discovery flags, explicit spec
sources, cross-origin trust, replacement behavior, non-interactive prompts,
spec refresh, list output, and inspection output.

When a spec omits `x-cli-config`, Restish can derive initial auth setup from
OpenAPI security requirements. Declared but unused `components.securitySchemes`
do not create prompts or credential bindings until an operation actually
references them.

API names become command groups. Names may contain Unicode letters, Unicode
numbers, combining marks, `-`, and `_`, and must start with a letter or number.
They cannot collide with built-in commands such as `api`, `get`, or `post`, or
with hidden compatibility commands such as `completion`. Removed commands are
not reserved; for example, `flags` is allowed as an API name in v2. API names
cannot contain whitespace, URL/path delimiters, or shell punctuation.

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
restish api auth inspect example
restish api auth add example PartnerKey
restish api auth remove example PartnerKey
restish api auth inspect example --rsh-credential basicAuth --redact
restish api auth inspect example --rsh-operation list-items --redact
restish api auth header example Authorization basicAuth
restish api auth header example Authorization --rsh-operation list-items
```

`api auth` manages profile credential bindings for generated OpenAPI
operations. `inspect` replaces the old top-level auth helper and
prints every configured credential by default. It also works for
non-Authorization credentials such as API-key headers. Use `--rsh-operation`
when an operation's OpenAPI security policy affects which credential applies.

## Related Pages

- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [Config](../config/)
- [Commands](../commands/)
