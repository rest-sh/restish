---
title: OAuth
linkTitle: OAuth
weight: 22
description: Choose and configure OAuth client credentials, authorization code, and device code auth in Restish.
---

Use this guide when an API uses OAuth and you need Restish to fetch, cache, and
refresh bearer tokens for a profile or generated OpenAPI credential.

OAuth setup is provider-specific. Restish owns the local CLI workflow, token
cache, request header, and OpenAPI credential binding. Your identity provider
still owns client registration, redirect URI rules, scopes, consent, and token
policy.

## Choose A Flow

| Flow | Restish type | Use it when |
| --- | --- | --- |
| Client credentials | `oauth-client-credentials` | A script, service account, CI job, or machine-to-machine integration calls the API. |
| Authorization code with PKCE | `oauth-authorization-code` | A human signs in through a browser and grants access to their account. |
| Device code | `oauth-device-code` | The terminal cannot receive a localhost browser callback, or the provider recommends device authorization for CLIs. |

Use `external-tool` instead when your organization already has an SSO helper or
request signer that should stay in charge of tokens.

Restish does not automatically switch an authorization-code profile into device
code, even when issuer discovery advertises both endpoints. Choose the flow that
matches the OAuth client registration and provider instructions. Use
`--rsh-no-browser` for a one-off browserless authorization-code sign-in; use
`oauth-device-code` when device authorization is the intended CLI flow.

## Store OAuth Config

For one API/profile, put OAuth directly under the profile's `auth` field. For
several APIs or several credentials, put the OAuth config in top-level
`auth_profiles` and reference it with `auth_ref`.

```jsonc
{
  "auth_profiles": {
    "work-user": {
      "type": "oauth-authorization-code",
      "params": {
        "issuer_url": "https://issuer.test",
        "client_id": "env:WORK_CLIENT_ID",
        "scopes": "read:items offline_access",
        "redirect_path": "/callback"
      }
    }
  },
  "apis": {
    "work": {
      "base_url": "https://api.vendor.test",
      "profiles": {
        "default": {
          "auth_ref": "work-user"
        }
      }
    }
  }
}
```

Prefer `env:NAME` for client IDs and secrets when the value should not live in
the config file. Keep the config file private because OAuth settings can still
describe sensitive tenants and clients.

## Use Issuer Discovery

When your provider has OpenID Connect discovery, prefer `issuer_url`:

```jsonc
{
  "type": "oauth-client-credentials",
  "params": {
    "issuer_url": "https://issuer.test",
    "client_id": "env:CLIENT_ID",
    "client_secret": "env:CLIENT_SECRET",
    "scopes": "read:items"
  }
}
```

Restish fetches `/.well-known/openid-configuration` under that issuer and uses
the advertised authorization, device authorization, and token endpoints as
needed. Discovery endpoints must stay under the issuer host and path scope.

If your provider does not publish discovery, configure direct endpoint URLs:

```jsonc
{
  "type": "oauth-client-credentials",
  "params": {
    "token_url": "https://issuer.test/oauth/token",
    "client_id": "env:CLIENT_ID",
    "client_secret": "env:CLIENT_SECRET",
    "audience": "https://api.vendor.test/"
  }
}
```

OAuth endpoints must use HTTPS except for localhost or loopback development
URLs. Endpoint URLs must not include credentials, fragments, or query strings.

When OAuth authorize, device authorization, or token endpoints live on the same
host as the API, you may use relative paths in manual config. `oauth2/token`
resolves under the active API/profile `base_url` path, while `/oauth2/token`
resolves at that host root. For example, with
`base_url: https://api.vendor.test/v1`, `oauth2/token` becomes
`https://api.vendor.test/v1/oauth2/token`.

## Browser Sign-In

Authorization code auth starts a local callback listener, opens the browser,
exchanges the returned code for a token, and caches the result.

```jsonc
{
  "type": "oauth-authorization-code",
  "params": {
    "authorize_url": "https://issuer.test/authorize",
    "token_url": "https://issuer.test/oauth/token",
    "client_id": "env:CLIENT_ID",
    "scopes": "read:items offline_access",
    "redirect_path": "/callback"
  }
}
```

Allow this callback URL in your OAuth app unless you change the port or path:

```text
http://localhost:8484/
```

With the example above, allow:

```text
http://localhost:8484/callback
```

Some providers distinguish `localhost` from `127.0.0.1`. Restish sends
`localhost` in the authorization request, so exact-match providers must allow
the `localhost` URL.

The browser callback page uses the active Restish theme. To brand that local
page, set `callback_success_html` and/or `callback_error_html` on the
authorization-code profile:

```jsonc
{
  "type": "oauth-authorization-code",
  "params": {
    "authorize_url": "https://issuer.test/authorize",
    "token_url": "https://issuer.test/oauth/token",
    "client_id": "env:CLIENT_ID",
    "callback_success_html": "<html><body><h1>Signed in</h1><p>You can return to the terminal.</p></body></html>",
    "callback_error_html": "<html><body><h1>Sign-in failed: $ERROR</h1><p>$DETAILS</p></body></html>"
  }
}
```

Callback HTML supports `$TITLE` and `$DETAILS`; failure HTML also supports
`$ERROR`. Restish escapes substituted values before inserting them, and does
not forward callback HTML params to OAuth authorization or token endpoints.

Use `--rsh-no-browser` when browser launch is not possible:

```bash
restish --rsh-no-browser myapi list-items
```

Restish prints the authorization URL and, when prompting is available, asks you
to paste the authorization code.

This keeps the configured flow as authorization code. It is useful over SSH, in
remote terminals, and on machines where opening the local browser is not
possible, as long as the provider allows copying the final authorization code
back into the terminal.

## Device Code

Device code auth prints the provider's verification instructions and polls the
token endpoint until you finish authorization or the provider's code expires.

```jsonc
{
  "type": "oauth-device-code",
  "params": {
    "issuer_url": "https://issuer.test",
    "client_id": "env:CLIENT_ID",
    "scopes": "read:items offline_access"
  }
}
```

Without discovery, set both `device_authorization_url` and `token_url`.

Choose device code when the provider documents it for CLI use, when a localhost
callback is impractical, or when the OAuth app is registered for device
authorization rather than redirect-based sign-in. Do not rely on Restish to
infer this from the issuer metadata; make the auth type explicit in config.

## Generated APIs

When an OpenAPI spec declares OAuth security schemes, `api connect` can prompt
for the required client details and write credential bindings under the active
profile.

```bash
restish api connect myapi https://api.vendor.test \
  'prompt.credentials.UserOAuth.client_id: env:CLIENT_ID' \
  'prompt.credentials.UserOAuth.scopes: read:items'
```

Generated commands use the operation's OpenAPI `security` requirements. If an
operation has several allowed alternatives, choose one explicitly:

```bash
restish myapi list-items --rsh-auth UserOAuth
restish myapi signed-report --rsh-auth UserOAuth+PartnerKey
```

Use `api auth inspect` to see configured credentials and whether OAuth access
tokens are cached:

```bash
restish api auth inspect myapi
```

## Token Cache

Restish caches OAuth tokens by API/profile or shared auth profile. Expired
access tokens are refreshed when a refresh token is available. If refresh fails
with `invalid_grant`, Restish clears that cached token and reruns the
interactive flow when the flow supports it.

If an API returns `401 Unauthorized` for a token-bearing OAuth request, Restish
forces fresh auth and retries that request once. This handles tokens that look
valid locally but were revoked, expired early, or rejected by the API's auth
layer. Restish does not keep retrying beyond that one controlled recovery; if
the retried request is still unauthorized, inspect or clear the cached auth
state.

Clear a cached OAuth token when consent changes, a grant is revoked, or you
want the next request to force a fresh sign-in:

```bash
restish api auth logout myapi
restish api auth logout myapi --all-profiles
restish api auth logout --auth-profile work-user
```

OAuth token cache is separate from HTTP response cache. `restish cache clear`
does not log you out.

## Provider Parameters

Restish forwards provider-specific OAuth params that are not reserved by the
flow. Common examples are:

| Param | Use |
| --- | --- |
| `audience` | Auth0-style resource server audience. |
| `resource` | Microsoft-style resource identifier. |
| `organization` | Tenant or organization hint used by some providers. |

Set `auth_method: client_secret_basic` when the provider expects the client
secret in an HTTP Basic token request header. The default is
`client_secret_post`, which sends the secret in the token request body.

## Troubleshooting

If Restish says an authorization-code credential has no cached access token,
rerun the command from an interactive terminal so the browser flow can complete
before the API request is sent.

If the provider does not return a refresh token, add the provider's offline
scope when required, often `offline_access`. Restish warns when a cached OAuth
flow receives an access token without a refresh token.

If discovery fails, confirm the issuer URL has no query string or fragment, and
that `/.well-known/openid-configuration` returns endpoints on the same issuer
host and path scope.

## Related Pages

- [Authentication](../authentication/)
- [Auth Reference](/docs/reference/auth/)
- [Profiles](/docs/reference/profiles/)
- [API Management](/docs/reference/api-management/)
- [Troubleshooting](../troubleshooting/)
- [Use External-Tool Auth](/docs/recipes/use-external-tool-auth/)
