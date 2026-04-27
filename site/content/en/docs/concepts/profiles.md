---
title: Profiles
linkTitle: Profiles
weight: 40
description: Understand how profiles layer request defaults for environments, auth, headers, query params, and TLS.
---

Profiles are named request defaults under an API. They let one command run in
different contexts without rewriting every URL, header, token, or TLS option.

## What Profiles Affect

A profile can provide:

- an environment-specific `base_url`
- default request `headers`
- default `query` parameters
- `auth` configuration
- TLS settings such as custom CA, client cert, or TLS signer plugin
- API command behavior such as operation-base or server-variable overrides

## Selection And Precedence

Use `-p` or `--rsh-profile` to select a profile:

```bash
restish -p json example list-images
```

Effective behavior is layered from config defaults, API config, selected
profile, environment variables, and finally command-line flags. The command
line wins for a single invocation.

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
        }
      }
    }
  }
}
```

## Good Profile Names

Use names that describe operating context, not implementation details:

- `default`
- `staging`
- `prod-readonly`
- `debug`
- `partner-a`

## Secrets

Auth settings may contain secrets. Keep config files private, prefer environment
references or external-tool auth where appropriate, and avoid putting reusable
secrets in shell history.

## Related Pages

- [Set Up Profiles](/docs/getting-started/set-up-profiles/)
- [Profiles Reference](/docs/reference/profiles/)
- [Authentication](/docs/guides/authentication/)
- [Config Reference](/docs/reference/config/)
