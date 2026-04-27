---
title: Use Multiple Profiles
linkTitle: Multiple Profiles
weight: 70
description: Switch request defaults with profile names.
---

Profiles are named defaults for an API. They keep commands readable when the
same API has different environments, auth, headers, query params, or TLS
settings. Start with small profiles and let command-line flags override them
when a single request needs to be different.

Config shape:

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "profiles": {
        "default": {},
        "json": { "headers": ["Accept: application/json"] },
        "debug": { "query": ["trace=docs"] }
      }
    }
  }
}
```

Use a profile:

```bash
restish -p json example list-images
```

The `json` profile adds an `Accept` header without making every command carry
that header by hand. The [Set Up Profiles](/docs/getting-started/set-up-profiles/)
tutorial walks through the same idea as part of the first-user path.

Related: [Set Up Profiles](/docs/getting-started/set-up-profiles/), [Profiles](/docs/reference/profiles/).
