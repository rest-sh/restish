---
title: Set Up Profiles
linkTitle: Set Up Profiles
weight: 45
description: Use profiles to switch environments, auth contexts, headers, query defaults, and TLS settings.
---

Profiles are named request defaults under an API. Use them when a command should
stay readable even though the target environment, auth, headers, query params,
or TLS settings change.

## Start With One API

```bash
restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'
restish config edit
```

Add profiles under the API:

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
        }
      }
    }
  }
}
```

Use a profile with `-p`:

```bash
restish -p json example list-images
restish -p debug https://api.rest.sh/anything/profile-demo
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

- [Profiles Concept](/docs/concepts/profiles/)
- [Profiles Reference](/docs/reference/profiles/)
- [Config Reference](/docs/reference/config/)
- [Authentication](/docs/guides/authentication/)
