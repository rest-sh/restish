---
title: Auth
linkTitle: Auth
weight: 23
description: Reference for Restish auth types, params, OpenAPI security bindings, secret sources, redaction, and inspection commands.
---

Auth configuration lives on a profile. Use `auth` when one credential applies
to the API/profile, and use `credentials` when generated OpenAPI operations
need different security schemes or alternatives.

## Auth Types

| Type | Required params | Optional params | Effect |
| --- | --- | --- | --- |
| `bearer` | `token` | | Sets `Authorization: Bearer <token>`. |
| `http-basic` | `username` | `password` | Sets HTTP Basic auth. If `password` is omitted and prompting is available, Restish prompts. |
| `api-key` | `in`, `name`, `value` | | Sends an API key in a `header`, `query`, or `cookie`. |
| `oauth-client-credentials` | `client_id`, `client_secret`, plus `token_url` or `issuer_url` | `auth_method`, `scopes`, provider-specific token params such as `audience` | Fetches and caches a bearer token with the OAuth client credentials flow. |
| `oauth-authorization-code` | `client_id`, plus `authorize_url` and `token_url`, or `issuer_url` | `client_secret`, `auth_method`, `scopes`, `redirect_scheme`, `redirect_port`, `redirect_path`, `redirect_cert`, `redirect_key`, `callback_success_html`, `callback_error_html`, provider-specific token params | Runs an OAuth authorization-code flow with PKCE and caches the token. |
| `oauth-device-code` | `client_id`, plus `device_authorization_url` and `token_url`, or `issuer_url` | `client_secret`, `auth_method`, `scopes`, provider-specific token params | Runs the OAuth device-code flow and caches the token. |
| `external-tool` | `commandline` | `omitbody`, `output` | Runs a local helper that can mutate request headers or URI. |

OAuth `auth_method` accepts `client_secret_post` by default or
`client_secret_basic`. OAuth endpoints must use HTTPS except for localhost or
loopback development URLs. `issuer_url` uses OIDC discovery when direct
endpoint URLs are absent. Unknown non-reserved OAuth params are forwarded to
token requests, which is how provider-specific values such as `audience` are
sent.

OAuth flow selection is explicit. Restish does not automatically switch an
`oauth-authorization-code` profile to `oauth-device-code` because issuer
discovery advertises a device endpoint. Use `oauth-device-code` when the OAuth
client is registered for device authorization. Use `--rsh-no-browser` with
`oauth-authorization-code` when you want the same redirect-based flow but need
to copy the authorization URL and pasted code manually.

For `oauth-authorization-code`, the default browser callback URL to allow in
the OAuth app is `http://localhost:8484/`. `redirect_port` changes `8484`, and
`redirect_path` changes `/`, for example `http://localhost:8484/callback`.
Some providers distinguish `localhost` from `127.0.0.1` or require loopback IP
redirects. Restish currently sends `localhost` in the authorization request, so
providers that perform exact redirect URI matching must allow the `localhost`
callback URL.

Set `redirect_scheme` to `https` when the OAuth app requires an HTTPS localhost
callback. HTTPS callbacks require `redirect_cert` and `redirect_key`, both PEM
paths used by the local callback server. Restish does not generate certificates
or install browser trust roots.

The browser callback page uses the active Restish theme by default.
`callback_success_html` and `callback_error_html` replace the full success or
failure page when an OAuth profile needs branded HTML. Callback HTML can
include `$TITLE` and `$DETAILS`; failure HTML also supports `$ERROR`. Restish
substitutes those placeholders with escaped callback values. These callback
HTML params are local UI settings and are not forwarded to OAuth authorization
or token endpoints.

## Profile Auth

Use profile-level `auth` for one effective credential:

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "profiles": {
        "token": {
          "auth": {
            "type": "bearer",
            "params": {
              "token": "env:EXAMPLE_TOKEN"
            }
          }
        }
      }
    }
  }
}
```

API-key auth needs a location, name, and value:

```jsonc
{
  "auth": {
    "type": "api-key",
    "params": {
      "in": "header",
      "name": "X-API-Key",
      "value": "env:EXAMPLE_API_KEY"
    }
  }
}
```

## OpenAPI Credentials

When a generated API has several security schemes, bind credentials by scheme
name under the active profile:

```jsonc
{
  "profiles": {
    "default": {
      "credentials": {
        "UserOAuth": {
          "auth_ref": "work-user-oauth",
          "satisfies": ["items:read"]
        },
        "PartnerKey": {
          "auth": {
            "type": "api-key",
            "params": {
              "in": "header",
              "name": "X-Partner-Key",
              "value": "env:PARTNER_KEY"
            }
          }
        }
      }
    }
  }
}
```

Each binding may use inline `auth` or `auth_ref`, not both. `auth_ref` points
to an entry in top-level `auth_profiles`. `satisfies` declares scopes or roles
that credential can cover when an operation has alternatives.

Generated commands use the operation's OpenAPI `security` policy. Generic URL
requests use the same operation policy when the selected API/profile has cached
OpenAPI metadata and one operation unambiguously matches the method and path.
Operations with `security: []` are public and suppress profile auth, auth
hooks, and sensitive credential headers/query values.

Use `--rsh-auth` to choose one allowed alternative:

```bash
restish myapi partner-report --rsh-auth PartnerKey
restish myapi signed-report --rsh-auth UserOAuth+PartnerKey
```

## Secret Sources

Auth params can reference environment variables with `env:NAME`:

```jsonc
{
  "type": "http-basic",
  "params": {
    "username": "demo",
    "password": "env:DEMO_PASSWORD"
  }
}
```

External command secret sources use `command:` in config fields that support
secret expansion. Those snippets run through `cmd /c` on Windows and
`/bin/sh -c` on other platforms; move complex logic into a script.

## External Tool Auth

`external-tool` auth sends a v1-compatible JSON request to a local helper on
stdin. The helper returns JSON describing header updates and an optional URI
rewrite:

```jsonc
{
  "auth": {
    "type": "external-tool",
    "params": {
      "commandline": "./scripts/sign-request",
      "omitbody": "true"
    }
  }
}
```

The tool receives:

```json
{"method":"GET","uri":"https://api.vendor.test/items","headers":{},"body":""}
```

It returns:

```json
{"headers":{"X-Signature":["abc123"]}}
```

Set `output` to `bearer-token` when stdout is a plain bearer token instead of
the JSON response shape. Restish records approved command hashes so a changed
external tool must be approved again.

## OAuth 401 Recovery

For token-bearing OAuth handlers, Restish may retry once after the target API
returns `401 Unauthorized`. The retry forces fresh auth state, then sends the
same request one more time. This recovers from cached access tokens that appear
unexpired locally but were revoked or rejected by the API. Restish does not
loop beyond the single auth recovery attempt; persistent failures should be
debugged with `api auth inspect` or cleared with `api auth logout`.

## Inspection And Redaction

Inspect configured auth without sending the target request:

```bash
restish api auth inspect myapi
restish api auth inspect myapi --credential PartnerKey
restish api auth inspect myapi --credential UserBearer --redact
restish api auth get myapi UserBearer
```

`api auth get` emits one curl-friendly fragment. Header credentials print as
`Name: value`; query credentials print as `?name=value`.

The bare form shows profile auth, configured credential readiness, generated
operation coverage when cached metadata is available, and computed auth material
when it can be built without starting a new OAuth flow. If a profile has several
credentials, `inspect` prints each configured credential's computed auth
material; pass `--credential` to narrow the output to one credential.
Inspection output shows computed auth values because the command is explicitly
for checking auth. Add `--redact` when you need shareable output; it is safe to
use even when no secrets are configured, so bug-report commands can include it
by default. Verbose request diagnostics still redact common sensitive headers
such as `Authorization`, `Cookie`, `Proxy-Authorization`, `Set-Cookie`, and
common API-key headers.

## Related Pages

- [Authentication Guide](/docs/guides/authentication/)
- [OAuth Guide](/docs/guides/oauth/)
- [Profiles](/docs/reference/profiles/)
- [Config](/docs/reference/config/)
- [API Management](/docs/reference/api-management/)
- [Commands](/docs/reference/commands/)
