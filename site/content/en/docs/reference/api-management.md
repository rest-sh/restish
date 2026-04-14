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

Common follow-up checks:

```bash
restish api list
restish api show <name>
restish <name> --help
```

### `api sync`

```bash
restish api sync <name>
```

Forces Restish to re-fetch the cached spec for a named API.

### `api list`

```bash
restish api list
```

Shows configured APIs.

### `api show`

```bash
restish api show <name>
```

Prints the saved config for one API as JSON.

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

### `api edit`

```bash
restish api edit
```

Opens `restish.json` in `$VISUAL` or `$EDITOR`.

### `api delete`

```bash
restish api delete <name>
```

Removes a configured API.

### `api clear-auth-cache`

```bash
restish api clear-auth-cache <name>
```

Deletes cached OAuth tokens for a named API.

### `api content-types`

```bash
restish api content-types
```

Lists the registered content types and MIME types known to the current Restish
process.

## `auth-header`

```bash
restish auth-header <api>
```

Prints the `Authorization` header value Restish would send for the selected API
and profile.

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

## `cert`

```bash
restish cert <uri>
restish cert --warn-days 14 <uri>
```

Inspects the TLS certificate chain presented by a server.

## Related Guides

- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [Authentication](/docs/guides/authentication/)
- [Retries and Caching](/docs/guides/retries-and-caching/)
- [TLS](/docs/guides/tls/)
