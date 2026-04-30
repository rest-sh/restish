---
title: Your First API Config
linkTitle: Your First Config
weight: 50
description: Learn the smallest useful restish.json shape before using the full config reference.
---

Most users create config with `restish api connect` and edit it only when they
need profiles, auth, or project-specific settings. This page shows the smallest
shape worth recognizing.

## Minimal Config

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "spec_url": "https://api.rest.sh/openapi.json"
    }
  }
}
```

That gives Restish a short API name, a base URL, and a spec URL for generated
commands.

## Add Profiles

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "spec_url": "https://api.rest.sh/openapi.json",
      "profiles": {
        "default": {},
        "json": {
          "headers": ["Accept: application/json"]
        }
      }
    }
  }
}
```

Use the profile:

```bash
restish -p json example list-images
```

## Editing Commands

```bash
restish api edit
restish api show example
restish api set example spec_url: https://api.rest.sh/openapi.json
restish api sync example
```

`api edit` preserves comments where possible. `api set` is better for small,
repeatable edits.

## Explicit Project Config

```bash
restish --rsh-config ./restish.json api connect example https://api.rest.sh
```

When `--rsh-config` or `RSH_CONFIG` is set, that file is the full config source
for the command.

## Related Pages

- [Set Up Profiles](../set-up-profiles/)
- [Config Reference](/docs/reference/config/)
- [API Management](/docs/reference/api-management/)
- [Docs Maintenance](/docs/contributing/docs-maintenance/)
