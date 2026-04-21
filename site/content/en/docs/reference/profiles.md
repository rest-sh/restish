---
title: Profiles
linkTitle: Profiles
weight: 25
description: Reference for Restish profile fields and how profile selection affects requests.
---

Profiles are the main unit of environment-specific behavior in Restish.

They live under an API config and let you vary:

- base URLs
- default headers
- default query parameters
- authentication
- TLS signer selection

They are the main way to model dev, staging, production, CI, and other
environment boundaries without rewriting every request command.

## Where Profiles Live

Profiles are defined under `apis.<name>.profiles`.

Example:

```json
{
  "apis": {
    "billing": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"]
        },
        "staging": {
          "base_url": "https://staging-api.example.com"
        }
      }
    }
  }
}
```

## Profile Fields

### `base_url`

Overrides the API-level `base_url` when this profile is active.

### `headers`

Persistent request headers in `Name: Value` format.

Example:

```json
{
  "headers": [
    "Accept: application/json",
    "X-Team: platform"
  ]
}
```

### `query`

Persistent `key=value` query parameters.

Example:

```json
{
  "query": [
    "per_page=100",
    "include=owner"
  ]
}
```

### `auth`

Authentication settings for the profile.

Shape:

```json
{
  "auth": {
    "type": "http-basic",
    "params": {
      "username": "alice"
    }
  }
}
```

Built-in auth types documented today:

- `http-basic`
- `oauth-client-credentials`
- `oauth-authorization-code`
- `external-tool`

`external-tool` is useful when a local script or helper must compute request
auth dynamically.

Example:

```json
{
  "auth": {
    "type": "external-tool",
    "params": {
      "commandline": "./scripts/sign-request.sh",
      "omitbody": "true"
    }
  }
}
```

### `tls_signer`

The name of a TLS signer plugin to use for mutual TLS.

### `tls_signer_params`

A string map of plugin-specific configuration passed to the TLS signer plugin.

Example:

```json
{
  "tls_signer": "restish-pkcs11",
  "tls_signer_params": {
    "module": "/usr/local/lib/opensc-pkcs11.so",
    "token_label": "YubiKey"
  }
}
```

## `operation_base`

`operation_base` is an API-level field (not inside a profile) that redirects
the URL prefix used when building requests from generated OpenAPI operations.

By default, Restish builds operation URLs from `base_url` plus the path in
the OpenAPI spec. Use `operation_base` when the spec's `servers` block differs
from the actual host, or when operations live on a different URL root than the
API itself.

Example:

```json
{
  "apis": {
    "billing": {
      "base_url": "https://billing.example.com",
      "operation_base": "https://billing.example.com/v1",
      "profiles": {}
    }
  }
}
```

With `operation_base` set, each OpenAPI path (e.g. `/invoices`) is appended to
`operation_base` rather than `base_url`.

Set `operation_base` via `restish api set`:

```bash
restish api set billing operation_base: "https://billing.example.com/v1"
```

Remove it with:

```bash
restish api set billing 'operation_base: undefined'
```

See also: [API Management Reference](../api-management/).

## Selecting a Profile

Choose a profile for one invocation:

```bash
restish -p staging get billing/invoices
```

Or set a persistent shell default:

```bash
export RSH_PROFILE=staging
```

Command-line `-p` takes precedence over `RSH_PROFILE`.

## Related Pages

- [Profiles Concept Guide](/docs/concepts/profiles/)
- [Authentication Guide](/docs/guides/authentication/)
- [Config Reference](../config/)
