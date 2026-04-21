---
title: API Management
linkTitle: API Management
weight: 18
description: Reference for configuring APIs, inspecting cache state, and managing generated command sources.
---

Restish has two layers of API management:

- configuration commands under `restish api`
- operational helpers such as `cache`, `auth-header`, and `cert`

## `restish api`

Use these commands to register and maintain named APIs.

### `api configure`

```bash
restish api configure <name> <url>
```

Registers an API and immediately tries to discover its OpenAPI description.

This is the usual starting point for API-aware commands.

Common follow-up checks:

```bash
restish api list
restish api show <name>
restish <name> --help
```

If discovery fails, the registration is still useful because the API short name
can still carry `base_url`, profiles, auth, and pagination config.

### `api sync`

```bash
restish api sync <name>
```

Forces Restish to re-fetch the cached spec for a named API.

Use this after the upstream API description changes.

### `api list`

```bash
restish api list
```

Shows configured APIs.

This is the fastest way to confirm that a registration exists.

### `api show`

```bash
restish api show <name>
```

Prints the saved config for one API as JSON.

This is useful when you need to confirm which `base_url`, `spec_url`, or
profiles are actually persisted.

### `api set`

```bash
restish api set <name> <key> <value>
```

Makes a narrow config edit using a dot-path key instead of opening the whole
file.

Typical uses:

```bash
restish api set github spec_url https://api.github.com/openapi.json
restish api set github base_url https://github.example.com/api/v3
```

This is best for single-field updates. If you are making several changes,
`api edit` is usually easier.

### `api edit`

```bash
restish api edit
```

Opens `restish.json` in `$VISUAL` or `$EDITOR`.

This is the best path when you need to:

- add several profiles
- restructure a large API config
- review comments in JSONC config

### `api delete`

```bash
restish api delete <name>
```

Removes a configured API.

### `api clear-auth-cache`

```bash
restish api clear-auth-cache <name>
restish api clear-auth-cache --all <name>
```

Deletes cached OAuth tokens for a named API.

Use this when an OAuth flow changed, a token was revoked, or you want to force
the next request back through the full token acquisition path.

Use `--all` when you want to clear every cached token for that API across all
profiles instead of only the active profile.

### `api content-types`

```bash
restish api content-types
```

Lists the registered content types and MIME types known to the current Restish
process.

This is especially useful when plugin-provided formatter or content-type support
is installed.

## `auth-header`

```bash
restish auth-header <api>
```

Prints the `Authorization` header value Restish would send for the selected API
and profile.

This is one of the best auth debugging commands because it uses the same auth
resolution path as a real request.

## Cache Commands

### `cache info`

```bash
restish cache info
```

Shows cache directory, total size, entry count, and oldest entry.

### `cache clear`

```bash
restish cache clear
restish cache clear <api>
```

Deletes cached responses globally or for one registered API.

Use the API-specific form when you want to invalidate one API without losing the
entire cache.

## `cert`

```bash
restish cert <uri>
restish cert --warn-days 14 <uri>
```

Inspects the TLS certificate chain presented by a server.

Use `--warn-days` when you want certificate-expiry checks in CI or operational
health checks.

## Related Guides

- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [Authentication](/docs/guides/authentication/)
- [Retries and Caching](/docs/guides/retries-and-caching/)
- [TLS](/docs/guides/tls/)
