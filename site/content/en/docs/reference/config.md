---
title: Config
linkTitle: Config
weight: 20
description: Reference for Restish configuration files, APIs, profiles, auth, cache, theme, and plugin settings.
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

## Top-Level Shape

```jsonc
{
  "apis": {},
  "profiles": {},
  "pagination": {},
  "cache": {},
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

## Editing

```bash
restish api edit
restish api set example spec_url: https://api.rest.sh/openapi.json
restish api show example
```

`api edit` preserves comments where possible. Use `api set` for small scripted
changes.

## Related Pages

- [Profiles](../profiles/)
- [Environment Variables](../environment-variables/)
- [API Management](../api-management/)
- [Security Design](/docs/contributing/design-records/)
