---
title: Your First API Config
linkTitle: First API Config
weight: 60
description: Learn the shape of your first restish.json API entry before diving into the full config reference.
---

If you only need one mental model for Restish config, use this one:

- the API registration names the service
- the profile selects the environment or auth context

## Smallest Useful Example

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"]
        }
      }
    }
  }
}
```

That is already enough to do useful work:

```bash
restish example/images
restish -p default example/images
```

## What Each Part Means

- `example`: the short API name you type on the command line
- `base_url`: the default host Restish should use for that API
- `profiles.default`: the normal request context for that API
- `headers`: persistent request headers applied whenever that profile is active

## A Slightly Richer Example

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "spec_url": "https://api.rest.sh/openapi.json",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"]
        },
        "debug": {
          "headers": [
            "Accept: application/json",
            "X-Debug: true"
          ]
        }
      }
    }
  }
}
```

This adds:

- `spec_url` so generated commands are predictable
- a second profile for a different request context

## The Useful Editing Commands

Start with interactive editing:

```bash
restish api edit
```

Use command helpers for narrower changes:

```bash
restish api configure example https://api.rest.sh
restish api set example spec_url https://api.rest.sh/openapi.json
restish api show example
```

For a project-local config, create the file first and select it explicitly:

```bash
printf '{}\n' > ./restish.json
restish --rsh-config ./restish.json api configure example https://api.rest.sh
```

`--rsh-config` and `RSH_CONFIG` select one complete config file. Restish does
not merge that file with your global config, and a missing explicit config file
is treated as an error.

## Where To Go Next

- [Set Up Profiles](../set-up-profiles/)
- [Connect to an API](../connect-to-an-api/)
- [Config Reference](/docs/reference/config/)
