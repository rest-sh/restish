---
title: Config
linkTitle: Config
weight: 20
description: Reference for the Restish configuration model and profile fields.
---

Restish v2 stores persistent configuration in a single `restish.json` file.

By default, that file lives in:

```text
~/.config/restish/restish.json
```

Restish accepts JSON with comments via JSONC parsing, but it still validates the
final shape strictly and rejects unknown fields.

Credentials may appear in this file (for example `auth.params.password` or
`auth.params.client_secret`). Restish writes config files with private file
permissions and warns when permissions are too open.

You can override the config directory with `RSH_CONFIG_DIR`.

OAuth token cache is stored as CBOR alongside config (default:
`~/.config/restish/tokens.cbor`). Restish reads legacy JSON token caches for
compatibility, writes CBOR on the next update, and rejects group/world-readable
token cache files.

## Top-Level Shape

```jsonc
{
  "apis": {},
  "cache": {},
  "theme": {},
  "plugins": {},
}
```

## `apis`

`apis` is a map from short API name to API config.

Example:

```jsonc
{
  "apis": {
    "github": {
      "base_url": "https://api.github.com",
    },
  },
}
```

### API Config Fields

- `base_url`: base URL for requests to that API
- `spec_url`: optional URL of the API spec
- `spec_files`: optional ordered list of local paths or URLs to merge as the
  spec source
- `operation_base`: optional absolute HTTP(S) URL prefix used for generated
  operation paths
- `profiles`: map of profile name to profile config
- `pagination`: optional pagination config

Use `spec_url` when the API publishes one main spec document.

Use `spec_files` when you want to merge several local or remote spec files in a
fixed order.

## `profiles`

Profiles live under an API, not globally.

### Profile Config Fields

- `base_url`: overrides the API-level `base_url`
- `headers`: persistent `Name: Value` headers
- `query`: persistent `key=value` query parameters
- `tls_signer`: name of the TLS signer plugin to use
- `tls_signer_params`: string map of plugin-specific TLS signer parameters
- `auth`: authentication config for the profile

### Auth Config Fields

- `type`: auth mechanism name such as `http-basic`,
  `oauth-client-credentials`, `oauth-authorization-code`, or `external-tool`
- `params`: string map of handler-specific parameters

## `pagination`

Pagination config lives under an API and supports:

- `items_path`: filter expression used to extract the collection items
- `next_path`: filter expression used to extract the next-page URL

## `cache`

Global cache settings currently support:

- `max_size`: maximum cache size, such as `100MB`

The HTTP response cache directory defaults to `~/.cache/restish/responses` and
can be overridden with `RSH_CACHE_DIR`.

## `theme`

`theme` stores terminal color/style entries used by readable output. You can
edit it directly or use:

```bash
restish theme set <url-or-user/repo> [name]
```

The value is a map of style names to style attributes. Use `theme set` when you
want Restish to fetch and install a shared theme definition.

## `plugins`

`plugins` stores per-plugin configuration keyed by plugin short name. The value
is raw JSON so Restish itself does not need to understand each plugin's
internal schema.

Example:

```jsonc
{
  "plugins": {
    "bulk": {
      "concurrency": 4,
      "retry": true,
    },
  },
}
```

## Example Full Config

```jsonc
{
  "apis": {
    "github": {
      "base_url": "https://api.github.com",
      "spec_url": "https://api.github.com/openapi.json",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"],
        },
        "enterprise": {
          "base_url": "https://github.example.com/api/v3",
          "query": ["per_page=100"],
          "auth": {
            "type": "http-basic",
            "params": {
              "username": "alice",
            },
          },
        },
      },
    },
  },
  "cache": {
    "max_size": "100MB",
  },
  "theme": {},
}
```

## Related Environment Variables

- `RSH_CONFIG_DIR`: override the config directory
- `RSH_CACHE_DIR`: override the HTTP response cache directory
- `RSH_PROFILE`: choose the default active profile
- `RSH_TIMEOUT`: set the default request timeout
- `RSH_RETRY`: set the default retry count for transient failures

## Editing Strategies

Use the config file directly when you want to make broad edits:

```bash
restish api edit
```

Use commands when you want small targeted changes:

```bash
restish api configure github https://api.github.com
restish api set github spec_url https://api.github.com/openapi.json
restish api show github
```

Use `api set` for narrow targeted edits. Use `api edit` when you need to
restructure several fields at once.

## Related Commands

- `restish api configure <name> <url>`
- `restish api show <name>`
- `restish api set <name> <key> <value>`
- `restish api edit`
- `restish api list`

## Related Pages

- [Your First API Config](/docs/getting-started/your-first-config/)
- [Profiles](../profiles/)
- [API Management](../api-management/)
