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

When an OpenAPI spec declares security schemes, `api configure` can prompt for
values or accept preanswers:

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: env:DOCS_API_KEY'
```

API keys may be stored as profile headers or query params under the hood, but
the setup flow should present them as API keys.

## Inspect The Final Header

For configured API auth, inspect the computed `Authorization` value without
making the full request:

```bash
restish auth-header example
```

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
      "scopes": ["read:items"]
    }
  }
}
```

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

## Related Pages

- [Profiles](/docs/reference/profiles/)
- [Config](/docs/reference/config/)
- [Inspect the Auth Header](/docs/recipes/inspect-the-auth-header/)
- [Use External-Tool Auth](/docs/recipes/use-external-tool-auth/)
- [Security Design](/docs/contributing/design-records/)
