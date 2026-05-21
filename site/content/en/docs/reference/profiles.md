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

<!-- BEGIN GENERATED: restish-docgen profile-schema -->
Generated from `internal/config/config.go`.

### `ProfileConfig`

ProfileConfig holds per-profile overrides for an API.

| JSON field | Go field | Type | Required | Description |
| --- | --- | --- | --- | --- |
| `base_url` | `BaseURL` | `string` | no | BaseURL overrides the API-level base_url when this profile is active. |
| `operation_base` | `OperationBase` | `string` | no | OperationBase overrides API-level operation_base when this profile is active. |
| `headers` | `Headers` | `[]string` | no | Headers is a list of persistent "Name: Value" headers sent with every request. |
| `query` | `Query` | `[]string` | no | Query is a list of persistent "key=value" query params sent with every request. |
| `ca_cert` | `CACertPath` | `string` | no | CACertPath is an optional PEM CA bundle for this profile. |
| `client_cert` | `ClientCertPath` | `string` | no | ClientCertPath is the PEM client certificate path for this profile. |
| `client_key` | `ClientKeyPath` | `string` | no | ClientKeyPath is the PEM client private key path for this profile. |
| `tls_signer` | `TLSSigner` | `string` | no | TLSSigner selects a tls-signer plugin for mTLS client certificate signing. |
| `tls_signer_params` | `TLSSignerParams` | `map[string]string` | no | TLSSignerParams passes plugin-specific configuration to the tls-signer. |
| `server_variables` | `ServerVariables` | `map[string]string` | no | ServerVariables overrides API-level OpenAPI server URL variables for this profile when generating operation paths. |
| `auth` | `Auth` | `*AuthConfig` | no | Auth holds authentication configuration for this profile. |
| `auth_ref` | `AuthRef` | `string` | no | AuthRef names a top-level auth_profiles entry to use for this profile. |
| `credentials` | `Credentials` | `map[string]*CredentialConfig` | no | Credentials maps operation credential requirement IDs to auth configurations that satisfy them. |

### `CredentialConfig`

CredentialConfig binds a local auth configuration to a generated operation credential requirement.

| JSON field | Go field | Type | Required | Description |
| --- | --- | --- | --- | --- |
| `auth` | `Auth` | `*AuthConfig` | no | Auth holds inline authentication configuration for this credential. |
| `auth_ref` | `AuthRef` | `string` | no | AuthRef names a top-level auth_profiles entry to use for this credential. |
| `satisfies` | `Satisfies` | `[]string` | no | Satisfies lists requirement values, such as OAuth scopes or non-OAuth roles, that this credential is intended to satisfy. |

### `AuthConfig`

AuthConfig holds authentication configuration for a profile.

| JSON field | Go field | Type | Required | Description |
| --- | --- | --- | --- | --- |
| `type` | `Type` | `string` | no | Type identifies the auth mechanism (e.g. "http-basic", "oauth-client-credentials"). |
| `params` | `Params` | `map[string]string` | no | Params holds handler-specific configuration, e.g. {"username": "alice"}. |
<!-- END GENERATED -->

Command-line flags override profile fields for one invocation.

## Auth

Use profile-level `auth` for APIs with one effective credential:

```bash
restish api set example \
  'profiles.token.auth: {type: bearer, params: {token: env:DOCS_TOKEN}}'
```

Common auth types include bearer tokens, basic auth, API keys, OAuth flows, and
external-tool auth. Auth params may reference environment variables with
`env:NAME` where supported. See [Auth](/docs/reference/auth/) for exact auth
types and params.

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

Use either file-backed mTLS (`client_cert` and `client_key`) or plugin-backed
mTLS (`tls_signer`) for a request. Restish rejects a profile or flag set that
combines a TLS signer with client certificate/key files.

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
