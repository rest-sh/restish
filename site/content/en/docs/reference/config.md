---
title: Config
linkTitle: Config
weight: 20
description: Reference for Restish configuration files, APIs, profiles, auth, cache, theme, and plugin settings.
---

Restish config is the trust boundary for API base URLs, profiles, auth, TLS,
plugins, and generated command sources.

## Location And Selection

Default config lives in the platform config directory as `restish.json`.
Override it explicitly:

```bash
restish --rsh-config ./restish.json api list
RSH_CONFIG=./restish.json restish api list
```

An explicit config file is the whole source of truth for that invocation.

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

Profiles can hold `base_url`, `headers`, `query`, `auth`, TLS fields, and plugin
settings. Auth params may contain secrets, so keep config permissions private.

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
