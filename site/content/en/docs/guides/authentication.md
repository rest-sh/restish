---
title: Authentication
linkTitle: Authentication
weight: 20
description: Configure auth in Restish using profiles and API-aware settings.
---

Restish v2 supports profile-driven authentication so repeated requests do not
require copying tokens and headers into every command.

Auth is configured under a profile, not directly on every command. That keeps
auth aligned with the same environment boundaries as base URLs, headers, and
other request defaults.

## Credential Storage Model

Restish may store auth-related secrets on disk:

- profile auth params in `restish.json` (for example passwords or client secrets)
- OAuth tokens in the CBOR token cache file (default `tokens.cbor` in the config directory)
- approved `external-tool` command hashes in `external-tool-approvals.json`

Keep your config directory private. Restish writes config files with private
permissions, warns if the config file becomes group/world-readable, and rejects
token or external-tool approval cache files with loose permissions.

## Start With The Principle

In Restish, auth is part of request setup, not a separate mode. That means:

- you usually configure it once under a profile
- the same request command can work across environments by switching profiles
- auth participates in the normal request pipeline instead of living in an ad hoc wrapper script

## Built-In Auth Types

Restish currently includes built-in support for:

- basic auth
- OAuth2 client credentials
- OAuth2 authorization code
- OAuth2 device code
- external-tool

If you need something more specialized, the auth hook/plugin model leaves room
for extension without turning every auth system into a core feature.

## Basic Auth Example

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "http-basic",
            "params": {
              "username": "alice",
              "password": "s3cr3t"
            }
          }
        }
      }
    }
  }
}
```

With that config, this command:

```bash
restish myapi/items
```

sends an `Authorization: Basic ...` header automatically.

That is the simplest useful pattern:

1. register an API
2. add auth under the profile
3. use ordinary Restish commands from there on

## API Keys

OpenAPI `apiKey` security schemes are set up as profile headers or query
parameters. You can let `api configure` prompt for the key, or pre-answer the
prompt:

```bash
restish api configure myapi api.example.com prompt.api_key: env:MYAPI_TOKEN
```

For a header-based API key scheme named `X-API-Key`, Restish writes a persistent
profile header such as `X-API-Key: env:MYAPI_TOKEN`. For a query API key, it
writes a persistent profile query parameter instead.

## Prompting For Secrets

You do not have to store every secret in config.

If a required secret such as a password is omitted:

```json
{
  "type": "http-basic",
  "params": {
    "username": "alice"
  }
}
```

Restish prompts for it when the request is made.

That gives you a useful middle ground:

- stable profile config in the file
- sensitive values entered interactively when needed

This is usually better than copying full auth headers into shell history.

## OAuth Notes

For token-based flows, Restish treats auth as request-time behavior:

- the selected profile chooses the auth context
- tokens are cached per `api:profile`
- the handler mutates the outbound request just before send

This keeps auth composable with the rest of the request pipeline instead of
hiding it inside the transport layer.

For users, the main effect is that OAuth-backed requests still feel like normal
Restish commands once the profile is configured.

### OAuth Client Credentials Example

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "ci": {
          "auth": {
            "type": "oauth-client-credentials",
            "params": {
              "client_id": "ci-client",
              "client_secret": "secret",
              "auth_method": "client_secret_basic",
              "token_url": "https://issuer.example.com/oauth/token",
              "scopes": "items.read items.write",
              "audience": "https://api.example.com/"
            }
          }
        }
      }
    }
  }
}
```

### OAuth Authorization Code Example

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "oauth-authorization-code",
            "params": {
              "client_id": "desktop-app",
              "authorize_url": "https://issuer.example.com/authorize",
              "token_url": "https://issuer.example.com/oauth/token",
              "scopes": "openid profile items.read",
              "organization": "acme"
            }
          }
        }
      }
    }
  }
}
```

Once configured, both still look like ordinary Restish requests:

```bash
restish -p ci myapi/items
restish -p default myapi/items
```

For authorization-code profiles, register the local redirect URL with your
provider as `http://localhost:8484/` unless you set a different `redirect_port`.
Restish includes the trailing slash in the OAuth `redirect_uri`.

### OAuth Auth Method And Extra Endpoint Params

OAuth token requests default to `client_secret_post`. If your IdP requires HTTP
Basic client authentication instead, set:

```json
{
  "type": "oauth-client-credentials",
  "params": {
    "client_id": "ci-client",
    "client_secret": "secret",
    "auth_method": "client_secret_basic"
  }
}
```

Any additional auth params not consumed directly by Restish are forwarded to
the OAuth endpoints. This is useful for IdP-specific fields such as:

- `audience`
- `resource`
- `organization`

### OAuth Device Code Example

Use device code when the machine running Restish cannot complete a browser
callback flow locally.

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "remote": {
          "auth": {
            "type": "oauth-device-code",
            "params": {
              "client_id": "device-client",
              "device_authorization_url": "https://issuer.example.com/oauth/device",
              "token_url": "https://issuer.example.com/oauth/token",
              "scopes": "openid profile items.read"
            }
          }
        }
      }
    }
  }
}
```

At request time, Restish prints the verification URL and user code, then polls
the token endpoint until login completes.

### Headless Authorization Code Fallback

If you still prefer authorization code flow on a remote machine, disable
automatic browser launch and paste the code manually:

```bash
restish --rsh-no-browser -p default myapi/items
```

When a TTY is attached, Restish prints the authorization URL and prompts for
the pasted code instead of trying to open a local browser. During normal browser
launches, the full authorization URL is only printed when browser launch fails
or verbose output is enabled.

## External Tool Example

Use `external-tool` when authentication has to be delegated to an existing
script, signer, or local credential helper.

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "external-tool",
            "params": {
              "commandline": "./scripts/sign-request.sh",
              "omitbody": "true"
            }
          }
        }
      }
    }
  }
}
```

At request time, Restish sends the outbound request to the tool as JSON on
stdin. The tool can return updated headers, an updated URI, or both.
The first time Restish sees a new `commandline` value, it prompts once and
stores the approved command hash in the config directory.

This is useful when:

- the auth system already exists as a script or local helper
- credentials must stay outside the Restish config file
- you need custom signing logic but do not need a full plugin

The tool receives the full outbound request, including any headers already
added earlier in request preparation. Treat it as trusted code.

OAuth discovery, authorization, device, and token HTTP requests use the same
TLS trust flags as ordinary requests, including `--rsh-ca-cert`,
`--rsh-tls-min-version`, and `--rsh-insecure`.

## A Common Pattern

It is common to keep multiple auth contexts under one API:

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "oauth-authorization-code"
          }
        },
        "ci": {
          "auth": {
            "type": "oauth-client-credentials",
            "params": {
              "client_id": "ci-client"
            }
          }
        }
      }
    }
  }
}
```

Then switch with:

```bash
restish -p default get myapi/items
restish -p ci get myapi/items
```

## Inspect The Final Header

If you want to see exactly what `Authorization` header Restish would send, use:

```bash
restish auth-header myapi
```

That command uses the same auth resolution path as a real request, which makes
it helpful for debugging config and shell integrations.

It is especially useful when you are trying to answer:

- which profile is active
- whether Restish is prompting for a missing secret
- whether an OAuth token is already cached

If you want to force a fresh token acquisition path, clear the cached token:

```bash
restish api clear-auth-cache myapi
restish api clear-auth-cache --all myapi
```

## Choosing Auth Per Environment

Because auth lives under profiles, switching environments also switches auth
cleanly:

- `default` might use personal credentials
- `ci` might use client credentials
- `enterprise` might use a different issuer or host entirely

Use `--rsh-profile` or `RSH_PROFILE` to choose the active profile.

## When To Reach For Plugins

If your auth system is not covered by the built-in handlers or `external-tool`,
use an auth plugin instead of trying to bolt custom header logic onto every
command.

That keeps authentication in the same request-time extension seam as the
built-in auth types.

## Learn More

- [Design Records](/docs/contributing/design-records/)
- [Profiles](../concepts/profiles/)
- [Set Up Profiles](/docs/getting-started/set-up-profiles/)
- [Plugin Quickstart](/docs/plugins/quickstart/)
