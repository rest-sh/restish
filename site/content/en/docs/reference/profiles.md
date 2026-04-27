---
title: Profiles
linkTitle: Profiles
weight: 21
description: Reference for profile fields and profile selection.
---

Profiles are named request defaults. They can be global or API-local; API-local
profiles are the common case.

## Example

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "profiles": {
        "default": {},
        "json": {
          "headers": ["Accept: application/json"]
        },
        "debug": {
          "query": ["trace=docs"],
          "headers": ["X-Debug: true"]
        }
      }
    }
  }
}
```

## Fields

| Field | Meaning |
| --- | --- |
| `base_url` | Override API base URL for this profile. |
| `headers` | Default request headers. |
| `query` | Default query params. |
| `auth` | Auth type and params. |
| `ca_cert`, `client_cert`, `client_key` | TLS file settings. |
| `tls_signer`, `tls_signer_params` | TLS signer plugin settings. |
| `operation_base` | Profile-specific operation path prefix. |
| `server_variables` | Profile-specific OpenAPI server variable values. |

## Selection

```bash
restish -p json example list-images
RSH_PROFILE=json restish example list-images
```

Command-line profile selection overrides `RSH_PROFILE`.

## Auth Notes

Auth params can reference environment variables where supported:

```jsonc
{
  "auth": {
    "type": "bearer",
    "params": { "token": "env:DOCS_TOKEN" }
  }
}
```

## Related Pages

- [Set Up Profiles](/docs/getting-started/set-up-profiles/)
- [Authentication](/docs/guides/authentication/)
- [Config](../config/)
