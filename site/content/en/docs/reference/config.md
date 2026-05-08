---
title: Config
linkTitle: Config
weight: 20
description: Reference for Restish configuration files, APIs, profiles, auth, cache, theme, and plugin settings.
aliases:
  - /docs/getting-started/your-first-config/
---

Restish config is the trust boundary for API base URLs, profiles, auth, TLS,
plugins, and generated command sources.

## Location And Selection

Default config lives in a Restish config directory as `restish.json`:

| Platform | Default path |
| --- | --- |
| macOS, Linux, and other Unix-like systems | `~/.config/restish/restish.json` |
| Windows | `%APPDATA%\restish\restish.json` |

Restish resolves config paths in this order:

1. `--rsh-config <file>`
2. `RSH_CONFIG=<file>`
3. `RSH_CONFIG_DIR=<dir>/restish.json`
4. `XDG_CONFIG_HOME/restish/restish.json`
5. the platform default above

Override the config file explicitly when a project, script, or test should not
use your user config:

```bash
restish --rsh-config ./restish.json api list
RSH_CONFIG=./restish.json restish api list
```

An explicit config file is the whole source of truth for that invocation. If
that file is missing, Restish errors instead of falling back to your user
config.

If Restish cannot determine a default config directory because the environment
has no usable `RSH_CONFIG_DIR`, `XDG_CONFIG_HOME`, platform user directory, or
`HOME`, set `RSH_CONFIG` or `RSH_CONFIG_DIR` explicitly. Restish will not create
a relative `./.restish` config directory in the current working directory.
Cache-only state may use a temporary directory until you set `RSH_CACHE_DIR` or
`XDG_CACHE_HOME`.

On Unix-like systems, Restish refuses to read a config file that is
group/world-readable because profiles and auth settings can contain secrets:

```bash
chmod 600 ~/.config/restish/restish.json
```

Inspect the active path from the CLI:

```bash
restish config path
restish config show
restish config show --json
```

## Top-Level Shape

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

## API Entries

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "spec_url": "https://api.rest.sh/openapi.json",
      "operation_base": "/",
      "command_layout": "flat",
      "retry_max_wait": "30s",
      "profiles": {
        "default": {},
        "json": { "headers": ["Accept: application/json"] }
      }
    }
  }
}
```

Common fields:

| Field | Meaning |
| --- | --- |
| `base_url` | Scheme and host for the API. |
| `spec_url` | Explicit OpenAPI document URL. |
| `spec_files` | Local spec files. |
| `operation_base` | Path prefix for operations. |
| `command_layout` | `flat` or `tags`. |
| `server_variables` | Configured OpenAPI server variable values. |
| `retry_max_wait` | API-local cap for `Retry-After`/`X-Retry-In` delays when no flag/env override is set. |
| `profiles` | API-local profiles. |

## Profiles And Auth

Profiles can hold `base_url`, `headers`, `query`, `auth`, `auth_ref`, TLS
fields, `server_variables`, and operation credential bindings. Auth params may
contain secrets, so keep config permissions private.

Credential bindings live under a profile and are keyed by the OpenAPI security
scheme or normalized credential requirement ID:

```jsonc
{
  "credentials": {
    "PartnerKey": {
      "auth": {
        "type": "api-key",
        "params": {
          "in": "header",
          "name": "X-Partner-Key",
          "value": "env:PARTNER_KEY"
        }
      }
    },
    "UserOAuth": {
      "auth_ref": "work-user-oauth",
      "satisfies": ["items:read"]
    }
  }
}
```

Each binding may use inline `auth` or `auth_ref`, not both. `satisfies` declares
the scopes or role values this local credential is allowed to cover.

OAuth authorization-code auth accepts `redirect_port` and `redirect_path` under
`auth.params`. `redirect_path` defaults to `/`, must start with `/`, and must
not include a scheme, host, query string, or fragment. This lets profiles match
provider registrations such as `http://localhost:8484/callback`.

Secret params may use `env:NAME` or `command:...`. Command secrets and
`external-tool` auth snippets run through `cmd /c` on Windows and `/bin/sh -c`
elsewhere, with bounded stderr redaction when a command fails.

## Editing

```bash
restish config edit
restish api set example 'spec_url: https://api.rest.sh/openapi.json'
restish api inspect example
```

`config edit` preserves comments where possible and prints the absolute config
file path after a successful write. Use `api set` for small scripted changes
inside one API registration.

`config set` and `api set` accept shorthand patch expressions. Objects merge
recursively, scalar values replace, `undefined` deletes fields or array items,
arrays support `[]` append and `[^index]` insertion, and `^` swaps values.
Restish validates the final patched config and reports multiple structural
issues when it can.

Create a profile with auth without opening an editor:

```bash
restish api set example \
  'profiles.demo.auth: {type: http-basic, params: {username: demo, password: env:EXAMPLE_PASSWORD}}'
```

The same change from `config set` uses the full config path:

```bash
restish config set \
  'apis.example.profiles.demo.auth: {type: http-basic, params: {username: demo, password: env:EXAMPLE_PASSWORD}}'
```

For global config fields, use `config set`:

```bash
restish config set 'cache.max_size: 250MB'
restish config theme set user/repo dark --yes
```

## Related Pages

- [Profiles](../profiles/)
- [Environment Variables](../environment-variables/)
- [API Management](../api-management/)
- [Security Design](/docs/contributing/design-records/)
