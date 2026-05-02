---
title: Authentication
linkTitle: Authentication
weight: 20
description: Configure Restish auth with profiles, generated API setup, safe auth fixtures, OAuth, and external tools.
---

Auth belongs with the request context. In Restish that usually means a profile,
so tokens, API keys, and environment-specific credentials do not get copied into
every command.

## Safe Auth Fixtures

The example API has endpoints that require auth and return only a safe summary.
They are useful for learning and testing.

### Bearer Token

```bash
restish -H 'Authorization: Bearer docs-token' https://api.rest.sh/auth/bearer
```

Representative output:

```readable
authenticated: true
scheme: "bearer"
subject: "docs-token"
```

### API Key Header

```bash
restish -H 'X-API-Key: docs-key' https://api.rest.sh/auth/api-key-header
```

### API Key Query Param

```bash
restish 'https://api.rest.sh/auth/api-key-query?api_key=docs-key'
```

### Basic Auth

```bash
restish -H 'Authorization: Basic YWxpY2U6c2VjcmV0' https://api.rest.sh/auth/basic
```

For real APIs, put these values in a profile or use an auth method instead of
repeating headers in shell history.

## Configure Auth In A Profile

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
restish api connect example https://api.rest.sh 'prompt.api_key: env:DOCS_API_KEY'
```

Generated OpenAPI commands use the operation's effective `security` policy.
For one global scheme, profile-level `auth` remains enough. For APIs with
several schemes, configure named credential bindings under the active profile:

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
sensitive credential headers/query values. Optional anonymous alternatives use
configured credentials when available and anonymous access as the fallback.

Choose a specific allowed alternative with `--rsh-auth`:

```bash
restish example partner-report --rsh-auth PartnerKey
restish example signed-report --rsh-auth UserOAuth+PartnerKey
```

## Inspect The Final Header

For configured API auth, inspect the computed auth material without making the
full request:

```bash
restish api auth inspect example
restish api auth inspect example --raw-header Authorization
restish api auth inspect example --rsh-credential PartnerKey
```

When a profile has exactly one configured credential, `inspect` selects it by
default. When a profile has several credentials, pass `--rsh-credential` so the
command knows which auth material to show.

Use verbose mode when the question is about the whole request:

```bash
restish -v -p token https://api.rest.sh/auth/bearer
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
path to `/`. Set `redirect_path` when your OAuth provider requires an exact
registered redirect URI such as `http://localhost:8484/callback`. Issuers and
OAuth endpoints must use HTTPS, except for localhost or loopback development
URLs.

Use `--rsh-no-browser` for headless sessions when the flow supports a manual
browser step.

## External Tool Auth

Use `external-tool` when another program owns credentials or signing:

```jsonc
{
  "auth": {
    "type": "external-tool",
    "params": {
      "command": ["./scripts/sign-request"]
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
- [Inspect the Auth Header](/docs/recipes/inspect-the-api auth inspect/)
- [Use External-Tool Auth](/docs/recipes/use-external-tool-auth/)
- [Security Design](/docs/contributing/design-records/)
