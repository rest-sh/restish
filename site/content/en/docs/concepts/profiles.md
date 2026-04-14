---
title: Profiles
linkTitle: Profiles
weight: 20
description: Learn how profiles let Restish reuse auth, TLS, and other settings across requests.
---

Profiles are the main way to reuse request settings across APIs and
environments. In Restish v2, profiles live under a specific API registration in
`restish.json`, not as a global concept shared across unrelated APIs.

That keeps the mental model simple:

- an API registration answers "what service is this?"
- a profile answers "which environment or auth context am I using for it?"

## Where Config Lives

By default, Restish stores config in:

```text
~/.config/restish/restish.json
```

You can override the config directory with `RSH_CONFIG_DIR`.

## What An API Registration Contains

At the API level, you can configure:

- `base_url`
- `spec_url`
- `spec_files`
- `operation_base`
- `profiles`
- `pagination`

Within each profile, you can configure:

- `base_url`
- `headers`
- `query`
- `auth`
- `tls_signer`
- `tls_signer_params`

That means profiles are the main place to encode environment-specific request
behavior without duplicating the whole API registration.

## Example

```jsonc
{
  "apis": {
    "github": {
      "base_url": "https://api.github.com",
      "spec_url": "https://api.github.com/openapi.json",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"]
        },
        "enterprise": {
          "base_url": "https://github.example.com/api/v3",
          "headers": ["Accept: application/json"],
          "query": ["per_page=100"]
        }
      }
    }
  }
}
```

## Common Profile Patterns

Profiles are especially useful for:

- `default` vs `staging` vs `production`
- personal credentials vs CI credentials
- public SaaS endpoints vs enterprise/self-hosted endpoints
- one API with different TLS signer or auth settings

## How Profile Selection Works

Restish chooses the active profile in this order:

1. `--rsh-profile`
2. `RSH_PROFILE`
3. `default`

That means you can keep a sane default while still switching explicitly for a
single command:

```bash
restish --rsh-profile enterprise get github/repos/octo/example
```

If neither the flag nor the environment variable is set, Restish falls back to
`default`.

## Override Rules

Profiles are durable defaults, not hard locks.

When Restish builds a request, the layering works like this:

1. the API registration establishes the base identity
2. the selected profile overrides API-level defaults such as `base_url`
3. profile headers and query parameters are added
4. command-line flags still win for that invocation

So this command:

```bash
restish --rsh-profile enterprise get github/repos/octo/example -q per_page=50
```

uses the `enterprise` profile, but the command-line `per_page=50` overrides the
profile's persistent `per_page=100`.

The same idea applies to headers, auth-adjacent flags, and other per-request
options: profiles provide durable defaults, but the command line remains the
last word for a single invocation.

## Editing Profiles

There are a few practical ways to manage them:

- `restish api edit` opens the whole config file in your editor
- `restish api show <name>` prints one API config as JSON
- `restish api set <name> <key> <value>` updates one field by dot-path

That gives you both a direct editing workflow and a few scripting-friendly
helpers.

## Strict Config Matters

Restish accepts JSON with comments, but it still validates the final shape
strictly and rejects unknown fields.

That is helpful when editing profiles by hand because typos fail early instead
of silently doing nothing.

## Why Profiles Matter

Profiles are the clean way to model common real-world situations:

- development vs staging vs production
- personal account vs service account
- public cloud vs enterprise/self-hosted endpoints
- the same API with different auth and TLS settings

## Related Commands

- `restish api list`
- `restish api show <name>`
- `restish api set <name> <key> <value>`
- `restish api edit`

## Learn More

- [Design Records](/docs/contributing/design-records/)
- [Authentication](../guides/authentication/)
- [Connect to an API](../getting-started/connect-to-an-api/)
- [Config Reference](../reference/config/)
