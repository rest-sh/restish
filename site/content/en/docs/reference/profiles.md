---
title: Profiles
linkTitle: Profiles
weight: 21
description: Reference for profile fields, profile selection, auth bindings, TLS settings, and precedence.
---

Profiles are named request defaults. Most profiles live under an API entry and
let the same command run against different environments, auth contexts, headers,
query params, or TLS settings.

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
        },
        "token": {
          "auth": {
            "type": "bearer",
            "params": {
              "token": "env:DOCS_TOKEN"
            }
          }
        }
      }
    }
  }
}
```

Use a profile with `-p` or `RSH_PROFILE`:

```bash
restish -p json example list-images
RSH_PROFILE=json restish example list-images
```

The command-line flag wins over `RSH_PROFILE`.

## Fields

| Field | Type | Meaning |
| --- | --- | --- |
| `base_url` | URL | Override the API base URL for this profile. |
| `headers` | array of `Name: Value` | Default request headers. |
| `query` | array of `key=value` | Default query parameters. |
| `auth` | object | Inline auth configuration. |
| `auth_ref` | string | Reference to a shared top-level auth profile. |
| `credentials` | object | Named OpenAPI security-scheme bindings. |
| `ca_cert` | path | Additional PEM CA certificate. |
| `client_cert` | path | PEM client certificate for mTLS. |
| `client_key` | path | PEM private key for mTLS. |
| `tls_signer` | string | TLS signer plugin name. |
| `tls_signer_params` | object or key/value params | Parameters for the TLS signer plugin. |
| `operation_base` | path | Profile-specific operation path prefix. |
| `server_variables` | object | OpenAPI server variable values. |

Command-line flags override profile fields for one invocation.

## Auth

Use profile-level `auth` for APIs with one effective credential:

```bash
restish api set example \
  'profiles.token.auth: {type: bearer, params: {token: env:DOCS_TOKEN}}'
```

Common auth types include bearer tokens, basic auth, API keys, OAuth flows, and
external-tool auth. Auth params may reference environment variables with
`env:NAME` where supported.

For generated APIs with several OpenAPI security schemes, use `credentials`.
Each key is a security scheme name or normalized credential requirement ID:

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

Each binding may use inline `auth` or `auth_ref`, not both. `satisfies`
declares scopes or roles that the credential can cover.

## TLS

TLS settings can live in a profile when they are part of the environment:

```bash
restish api set internal \
  'profiles.prod.ca_cert: ./corp-ca.pem' \
  'profiles.prod.client_cert: ./client.pem' \
  'profiles.prod.client_key: ./client-key.pem'
```

For external key signing, use `tls_signer` and signer params:

```bash
restish api set internal \
  'profiles.hsm.tls_signer: pkcs11' \
  'profiles.hsm.tls_signer_params.module: /usr/local/lib/opensc-pkcs11.so'
```

## Precedence

Effective request behavior is layered:

1. built-in defaults
2. API config
3. selected profile
4. environment variables such as `RSH_PROFILE`
5. command-line flags

Use profiles for repeated context. Use flags for one-off overrides.

## Errors

Restish errors when a requested profile is missing, when a profile name is
invalid, when auth material cannot be resolved, or when a generated operation
requires a credential binding the selected profile does not provide.

## Related Pages

- [Set Up Profiles](/docs/getting-started/set-up-profiles/)
- [Authentication](/docs/guides/authentication/)
- [Config](../config/)
- [Global Flags](../global-flags/)
