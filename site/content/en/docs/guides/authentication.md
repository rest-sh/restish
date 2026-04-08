---
title: Authentication
linkTitle: Authentication
weight: 20
description: Configure auth in Restish using profiles and API-aware settings.
---

# Authentication

Restish v2 supports profile-driven authentication so repeated requests do not
require copying tokens and headers into every command.

Auth is configured under a profile, not directly on every command. That keeps
auth aligned with the same environment boundaries as base URLs, headers, and
other request defaults.

## Built-In Auth Types

Restish currently includes built-in support for:

- basic auth
- OAuth2 client credentials
- OAuth2 authorization code

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
restish get myapi/items
```

sends an `Authorization: Basic ...` header automatically.

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

## OAuth Notes

For token-based flows, Restish treats auth as request-time behavior:

- the selected profile chooses the auth context
- tokens are cached per `api:profile`
- the handler mutates the outbound request just before send

This keeps auth composable with the rest of the request pipeline instead of
hiding it inside the transport layer.

## Inspect The Final Header

If you want to see exactly what `Authorization` header Restish would send, use:

```bash
restish auth-header myapi
```

That command uses the same auth resolution path as a real request, which makes
it helpful for debugging config and shell integrations.

## Choosing Auth Per Environment

Because auth lives under profiles, switching environments also switches auth
cleanly:

- `default` might use personal credentials
- `ci` might use client credentials
- `enterprise` might use a different issuer or host entirely

Use `--rsh-profile` or `RSH_PROFILE` to choose the active profile.

## Learn More

- [`docs/design/004-authentication.md`](/Users/daniel/src/restish2/docs/design/004-authentication.md)
- [Profiles](../concepts/profiles/)
