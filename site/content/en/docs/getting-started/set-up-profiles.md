---
title: Set Up Profiles
linkTitle: Set Up Profiles
weight: 50
description: Set up profiles so Restish can switch between environments, auth contexts, and defaults cleanly.
---

Once you have an API registered, profiles are the feature that makes Restish
feel practical every day.

Use profiles when you want one API definition but different:

- environments such as `staging` and `production`
- auth contexts such as personal and CI credentials
- default headers or query params
- TLS signer settings

## Why Do This Early

Without profiles, repeated API work tends to look like this:

```bash
restish https://api.rest.sh/images -H 'Accept: application/json'
restish https://api.rest.sh/images -H 'X-Debug: true'
```

With profiles, the repeated context moves into config and the command becomes
about the request itself instead of the environment setup.

## Example

```json
{
  "apis": {
    "billing": {
      "base_url": "https://api.rest.sh",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"]
        },
        "debug": {
          "headers": ["Accept: application/json", "X-Debug: true"]
        }
      }
    }
  }
}
```

Then choose a profile with:

```bash
restish -p debug get billing/images
restish -p default get billing/images
```

## What Profiles Can Hold

Profiles live under one API and can include:

- `base_url`
- `headers`
- `query`
- `auth`
- `tls_signer`
- `tls_signer_params`

That makes them the right place for environment-specific defaults.

## A Practical Workflow

1. register the API with `restish api configure <name> <url>`
2. open the config with `restish api edit`
3. add a `profiles` map under that API
4. run requests with `-p <profile>`

If you prefer to keep one profile active for a while, set:

```bash
export RSH_PROFILE=debug
```

## Keep The Command Line In Charge

Profiles are defaults, not hard locks. Command-line flags still win for one
invocation.

That means you can keep a stable profile and still override a detail when you
need to:

```bash
restish -p debug get billing/images -q format=jpeg
```

## Good First Profiles To Create

- `default`
- `debug`
- `ci`

That small set usually covers local use, pre-production testing, and
automation.

## What To Read Next

- [Profiles Concept Guide](/docs/concepts/profiles/)
- [Authentication Guide](/docs/guides/authentication/)
- [Use Multiple Profiles Recipe](/docs/recipes/use-multiple-profiles/)
