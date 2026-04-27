---
title: Use Multiple Profiles
linkTitle: Multiple Profiles
weight: 70
description: Switch request defaults with profile names.
---

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

Related: [Set Up Profiles](/docs/getting-started/set-up-profiles/), [Profiles](/docs/reference/profiles/).
