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

### `query`

Persistent `key=value` query parameters.

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

### `tls_signer`

The name of a TLS signer plugin to use for mutual TLS.

### `tls_signer_params`

A string map of plugin-specific configuration passed to the TLS signer plugin.

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
