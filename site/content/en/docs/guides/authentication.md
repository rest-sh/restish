---
title: Authentication
linkTitle: Authentication
weight: 20
description: Configure Restish auth with profiles, generated API setup, safe auth fixtures, OAuth, and external tools.
aliases:
  - /docs/recipes/send-an-api-key-in-a-header/
  - /docs/recipes/send-an-api-key-in-a-query-parameter/
  - /docs/recipes/inspect-the-auth-header/
---

Auth belongs with the request context. In Restish that usually means a profile,
so tokens, API keys, and environment-specific credentials do not get copied into
every command.

## Safe Auth Fixtures

The example API has endpoints that require auth and return only a safe summary.
They are useful for learning and testing.

### Bearer Token

{{< restish-example >}}
restish -H 'Authorization: Bearer docs-token' api.rest.sh/auth/bearer
{{< /restish-example >}}

Representative output:

```readable
authenticated: true
scheme: "bearer"
subject: "docs-token"
```

### API Key Header

{{< restish-example >}}
restish -H 'X-API-Key: docs-key' api.rest.sh/auth/api-key-header
{{< /restish-example >}}

### API Key Query Param

{{< restish-example >}}
restish 'api.rest.sh/auth/api-key-query?api_key=docs-key'
{{< /restish-example >}}

### Basic Auth

{{< restish-example >}}
restish -H 'Authorization: Basic YWxpY2U6c2VjcmV0' api.rest.sh/auth/basic
{{< /restish-example >}}

For real APIs, put these values in a profile or use an auth method instead of
repeating headers in shell history.

## Configure Auth In A Profile

Use `api set` to create or update profile auth without opening the config file:

```bash
restish api set example \
  'profiles.token.auth: {type: bearer, params: {token: env:RESTISH_DOCS_TOKEN}}'
```

That writes the same shape you would see in `restish.json`:

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
              "token": "env:RESTISH_DOCS_TOKEN"
            }
          }
        }
      }
    }
  }
}
```

Then call the API:

```bash
RESTISH_DOCS_TOKEN=docs-token restish -p token example get-echo
```

## Prompt-Driven Setup

When an OpenAPI spec declares security schemes, `api connect` can prompt for
values or accept preanswers:

```bash
restish api connect example api.rest.sh 'prompt.api_key: env:DOCS_API_KEY'
```

Generated OpenAPI commands use the operation's effective `security` policy.
Generic URL requests use the same policy when their method and URL path
unambiguously match one cached operation for the selected API/profile. For one
global scheme, profile-level `auth` remains enough. For APIs with several
schemes, configure named credential bindings under the active profile:

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

`security: []` operations are public and suppress profile auth, auth hooks, and
sensitive credential headers/query values, including for matching generic URL
requests. Optional anonymous alternatives use configured credentials when
available and anonymous access as the fallback.

For an API that defines several allowed auth alternatives, choose one with
`--rsh-auth`:

```bash
restish myapi partner-report --rsh-auth PartnerKey
restish myapi signed-report --rsh-auth UserOAuth+PartnerKey
```

These commands use placeholder operation names because the public example API
does not expose a multi-scheme partner-auth fixture.

## Inspect The Final Header

For configured API auth, inspect the computed auth material without making the
full request:

```bash
restish api auth inspect myapi
restish api auth inspect myapi --rsh-credential PartnerKey
restish api auth inspect myapi --rsh-credential UserBearer --raw-header Authorization
```

When a profile has exactly one configured credential, `inspect` selects it by
default. When a profile has several credentials, pass `--rsh-credential` so the
command knows which auth material to show.

Use verbose mode when the question is about the whole request:

```bash
restish -v -p token api.rest.sh/auth/bearer
```

## OAuth

OAuth examples require your provider, redirect settings, client ID, and client
secret, so the docs use placeholder issuer hosts. Keep those examples out of
shell history when they contain secrets.

Client credentials shape:

```jsonc
{
  "auth": {
    "type": "oauth-client-credentials",
    "params": {
      "token_url": "https://issuer.test/oauth/token",
      "client_id": "env:CLIENT_ID",
      "client_secret": "env:CLIENT_SECRET",
      "audience": "https://api.vendor.test/"
    }
  }
}
```

Authorization code shape:

```jsonc
{
  "auth": {
    "type": "oauth-authorization-code",
    "params": {
      "authorize_url": "https://issuer.test/authorize",
      "token_url": "https://issuer.test/oauth/token",
      "client_id": "env:CLIENT_ID",
      "scopes": "read:items",
      "redirect_path": "/callback"
    }
  }
}
```

Restish opens a localhost callback on `redirect_port` and defaults the callback
path to `/`.

Allow this callback URL in your OAuth app by default:

```text
http://localhost:8484/
```

If you set `redirect_path`, allow the matching path instead:

```text
http://localhost:8484/callback
```

If you set `redirect_port`, allow the same path on that port. Some OAuth
providers distinguish `localhost` from `127.0.0.1` or require loopback IP
redirects such as `http://127.0.0.1:8484/`. Restish currently sends
`localhost` in the authorization request, so the provider must allow the
`localhost` URL exactly for the browser callback flow to complete.

Issuers and OAuth endpoints must use HTTPS, except for localhost or loopback
development URLs.

Use `--rsh-no-browser` for headless sessions when the flow supports a manual
browser step.

## External Tool Auth

Use `external-tool` when another program owns credentials or signing:

```jsonc
{
  "auth": {
    "type": "external-tool",
    "params": {
      "commandline": "./scripts/sign-request"
    }
  }
}
```

Restish records approved command hashes so a changed executable must be trusted
again.

External-tool and `command:` secret sources run through `cmd /c` on Windows and
`/bin/sh -c` on other platforms. Keep snippets portable, avoid relying on your
interactive `$SHELL`, and move complex logic into a script.

## Related Pages

- [Profiles](/docs/reference/profiles/)
- [Config](/docs/reference/config/)
- [Auth Reference](/docs/reference/auth/)
- [Use External-Tool Auth](/docs/recipes/use-external-tool-auth/)
- [Security Design](/docs/contributing/design-records/)
