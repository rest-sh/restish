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

You can override the config directory with `RSH_CONFIG_DIR`.

## Top-Level Shape

```json
{
  "apis": {},
  "cache": {},
  "allowed_plugins": [],
  "plugins": {}
}
```

## `apis`

`apis` is a map from short API name to API config.

Example:

```json
{
  "apis": {
    "github": {
      "base_url": "https://api.github.com"
    }
  }
}
```

### API Config Fields

- `base_url`: base URL for requests to that API
- `spec_url`: optional URL of the API spec
- `spec_files`: optional ordered list of local paths or URLs to merge as the
  spec source
- `operation_base`: optional URL prefix used for generated operation paths
- `profiles`: map of profile name to profile config
- `pagination`: optional pagination config

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
  `oauth-client-credentials`, or `oauth-authorization-code`
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

## `allowed_plugins`

When non-empty, this limits plugin auto-discovery to specific executable base
names such as:

```json
{
  "allowed_plugins": ["restish-bulk", "restish-csv"]
}
```

## `plugins`

`plugins` stores per-plugin configuration keyed by plugin short name. The value
is raw JSON so Restish itself does not need to understand each plugin's
internal schema.

Example:

```json
{
  "plugins": {
    "bulk": {
      "concurrency": 4,
      "retry": true
    }
  }
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
          "headers": ["Accept: application/json"]
        },
        "enterprise": {
          "base_url": "https://github.example.com/api/v3",
          "query": ["per_page=100"],
          "auth": {
            "type": "http-basic",
            "params": {
              "username": "alice"
            }
          }
        }
      }
    }
  },
  "cache": {
    "max_size": "100MB"
  }
}
```

## Related Environment Variables

- `RSH_CONFIG_DIR`: override the config directory
- `RSH_CACHE_DIR`: override the HTTP response cache directory
- `RSH_PROFILE`: choose the default active profile

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

## Related Commands

- `restish api configure <name> <url>`
- `restish api show <name>`
- `restish api set <name> <key> <value>`
- `restish api edit`
- `restish api list`

Primary source:

- [`docs/design/002-config-and-profiles.md`](/docs/contributing/design-records/)
