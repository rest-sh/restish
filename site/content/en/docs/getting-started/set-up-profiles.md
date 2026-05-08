---
title: Set Up Profiles
linkTitle: Set Up Profiles
weight: 45
description: Use profiles to switch environments, auth contexts, headers, query defaults, and TLS settings.
aliases:
  - /docs/recipes/use-multiple-profiles/
---

Profiles are named request defaults under an API. Use them when a command should
stay readable even though the target environment, auth, headers, query params,
or TLS settings change.

## Start With One API

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
```

Add profiles under the API with `api set`:

```bash
restish api set example 'profiles.json.headers[]: "Accept: application/json"'
restish api set example \
  'profiles.debug: {headers: ["Accept: application/json", "X-Debug: true"], query: ["trace=docs"]}'
restish api set example \
  'profiles.token.auth: {type: bearer, params: {token: env:RESTISH_DOCS_TOKEN}}'
```

Those commands write profile entries like this:

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
          "headers": ["Accept: application/json", "X-Debug: true"],
          "query": ["trace=docs"]
        },
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

Use a profile with `-p`:

```bash
restish -p json example list-images
restish -p debug api.rest.sh/anything/profile-demo
RESTISH_DOCS_TOKEN=docs-token restish -p token example get-echo
```

## What Profiles Can Hold

Profiles can provide:

- `base_url` overrides for environments
- default `headers`
- default `query` parameters
- `auth` settings
- TLS settings such as client certs, custom CA, or TLS signer plugin
- operation-base and server-variable overrides where configured

Command-line flags still win for the current invocation. Profiles are defaults,
not a prison.

## A Practical Pattern

Use profiles for stable contexts:

- `default` for the environment most people use
- `staging` for pre-production
- `prod-readonly` for production commands with safer auth
- `debug` for extra headers or query params used during investigation

Then keep commands small:

```bash
restish -p staging myapi list-users
restish -p prod-readonly myapi get-user 123
```

## Secrets

Profiles can contain secrets, especially for basic auth, API keys, and OAuth
client credentials. Keep config files private and prefer environment-variable
references or external tools when your team does not want secrets in
`restish.json`.

## Next Step

Use [Requests](/docs/guides/requests/) for the broader daily workflow, or jump
to [Authentication](/docs/guides/authentication/) when credentials are your next
task.

## Related Pages

- [Profiles Reference](/docs/reference/profiles/)
- [Profiles Reference](/docs/reference/profiles/)
- [Config Reference](/docs/reference/config/)
- [Authentication](/docs/guides/authentication/)
