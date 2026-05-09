---
title: Config
linkTitle: Config
weight: 20
description: Reference for Restish configuration files, APIs, profiles, auth, cache, themes, plugins, and precedence.
aliases:
  - /docs/getting-started/your-first-config/
---

Restish config is the trust boundary for API base URLs, generated command
sources, profiles, auth, TLS, plugins, cache settings, and readable-output
themes.

## Location And Selection

Default config lives in a Restish config directory as `restish.json`:

| Platform | Default path |
| --- | --- |
| macOS, Linux, and other Unix-like systems | `~/.config/restish/restish.json` |
| Windows | `%APPDATA%\restish\restish.json` |

Config path precedence:

1. `--rsh-config <file>`
2. `RSH_CONFIG=<file>`
3. `RSH_CONFIG_DIR=<dir>/restish.json`
4. `XDG_CONFIG_HOME/restish/restish.json`
5. platform default

```bash
restish --rsh-config ./restish.json api list
RSH_CONFIG=./restish.json restish api list
```

An explicit config file is the whole source of truth for that invocation. If it
is missing, Restish errors instead of falling back to user config.

If no default config directory can be determined, set `RSH_CONFIG` or
`RSH_CONFIG_DIR`. Restish does not create a relative `./.restish` directory.
Cache-only state may use a temporary directory until `RSH_CACHE_DIR` or
`XDG_CACHE_HOME` is available.

On Unix-like systems, Restish refuses group/world-readable config files because
profiles and auth settings may contain secrets:

```bash
chmod 600 ~/.config/restish/restish.json
```

Inspect the active config:

```bash
restish config path
restish config show
restish config show --json
```

## Top-Level Fields

```jsonc
{
  "apis": {},
  "auth_profiles": {},
  "cache": {},
  "theme_source": "https://example.com/theme.json",
  "theme": {},
  "plugins": {}
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `apis` | object | Registered APIs keyed by short name. |
| `auth_profiles` | object | Shared auth profiles referenced by `auth_ref`. |
| `cache` | object | HTTP response cache settings. |
| `theme_source` | URL or GitHub shorthand | Source used by `config theme set`. |
| `theme` | object | Readable-output highlighting theme. |
| `plugins` | object | Installed plugin metadata and trust decisions. |

## API Entries

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "spec_url": "https://api.rest.sh/openapi.json",
      "spec_files": [],
      "operation_base": "/",
      "command_layout": "flat",
      "server_variables": {},
      "retry_max_wait": "30s",
      "profiles": {
        "default": {},
        "json": { "headers": ["Accept: application/json"] }
      }
    }
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `base_url` | URL | Scheme, host, and optional base path for the API. |
| `spec_url` | URL or file path | Explicit OpenAPI document source. |
| `spec_files` | array | Local OpenAPI files merged into the API description. |
| `operation_base` | path | Path prefix override for generated operations. |
| `command_layout` | `flat` or `tags` | Generated command grouping. |
| `server_variables` | object | OpenAPI server variable values. |
| `retry_max_wait` | duration | API-local cap for `Retry-After` or `X-Retry-In` when no flag/env override is set. |
| `profiles` | object | API-local profiles. |

API names may contain Unicode letters, Unicode numbers, combining marks, `-`,
and `_`, and must start with a letter or number. They cannot collide with
built-in commands such as `api`, `get`, or `post`.

## Profiles

Profiles hold request defaults under an API. See [Profiles](../profiles/) for
the full field reference.

```jsonc
{
  "profiles": {
    "default": {},
    "staging": {
      "base_url": "https://staging.example.com",
      "headers": ["Accept: application/json"],
      "query": ["trace=docs"],
      "auth": {
        "type": "bearer",
        "params": {
          "token": "env:EXAMPLE_TOKEN"
        }
      }
    }
  }
}
```

## Auth Profiles

Use top-level `auth_profiles` when several APIs or profiles should share one
credential configuration:

```jsonc
{
  "auth_profiles": {
    "work-user-oauth": {
      "type": "oauth-authorization-code",
      "params": {
        "authorize_url": "https://issuer.test/authorize",
        "token_url": "https://issuer.test/oauth/token",
        "client_id": "env:CLIENT_ID",
        "scopes": "read:items",
        "redirect_path": "/callback"
      }
    }
  }
}
```

Profiles can reference these with `auth_ref`.

Secret params may use `env:NAME` or `command:...`. Command secrets and
`external-tool` auth snippets run through `cmd /c` on Windows and `/bin/sh -c`
elsewhere.

## Cache

Cache settings control the HTTP response cache, not OAuth/auth token cache.
Use `restish cache info` to inspect runtime cache location, size, entry count,
and oldest entry.

```bash
restish config set 'cache.max_size: 250MB'
restish cache info
restish cache clear
```

Use `restish api auth logout` for cached auth tokens.

## Theme

Themes affect `readable` output only:

```bash
restish config theme set ./themes/one-dark-pro.json
restish config theme set user/repo dark --yes
```

`theme_source` records where the theme came from. Local paths are stored as
absolute paths. `theme` stores the resolved highlighting values. Use
`header_key` to color HTTP response header names differently from JSON/readable
object keys.

## Plugins

The `plugins` object stores installed plugin metadata and trust decisions.
Users normally manage it through:

```bash
restish plugin list
restish plugin install rest-sh/restish:csv
restish plugin remove restish-csv
```

Installed plugins are trusted local executables. Restish checks manifests and
declared capabilities, but it does not sandbox plugin code.

## Editing

Use `config edit` for larger changes:

```bash
restish config edit
```

Use `api set` for API-scoped patches:

```bash
restish api set example 'spec_url: https://api.rest.sh/openapi.json'
restish api set example \
  'profiles.demo.auth: {type: http-basic, params: {username: demo, password: env:EXAMPLE_PASSWORD}}'
```

Use `config set` for global patches:

```bash
restish config set \
  'apis.example.profiles.demo.auth: {type: http-basic, params: {username: demo, password: env:EXAMPLE_PASSWORD}}'
restish config set 'cache.max_size: 250MB'
```

`config set` and `api set` use shorthand patch expressions. Objects merge
recursively, scalars replace, `undefined` deletes fields or array items, `[]`
appends, `[^index]` inserts before an array index, and `^` moves or swaps
values. Restish validates the final config before writing.

## Precedence

Request behavior is layered from lower to higher precedence:

1. built-in defaults
2. API config
3. selected profile
4. environment variables
5. command-line flags

Explicit config selection is not layered. `--rsh-config` or `RSH_CONFIG`
selects one config file for the invocation.

## Related Pages

- [Profiles](../profiles/)
- [Environment Variables](../environment-variables/)
- [API Management](../api-management/)
- [Shorthand](../shorthand/)
- [Security Design](/docs/contributing/design-records/)
