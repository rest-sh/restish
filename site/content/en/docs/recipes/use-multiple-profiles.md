---
title: Use Multiple Profiles
linkTitle: Use Multiple Profiles
weight: 20
description: Switch between environments or credentials with Restish profiles.
---

Profiles are the clean way to move between dev, staging, and production
environments without rewriting every command.

Example config:

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "query": ["per_page=50"]
        },
        "staging": {
          "base_url": "https://staging-api.example.com",
          "query": ["per_page=20"]
        },
        "ci": {
          "auth": {
            "type": "oauth-client-credentials",
            "params": {
              "client_id": "ci-client"
            }
          }
        }
      }
    }
  }
}
```

Use a profile for one command:

```bash
restish -p staging get myapi/items
restish -p ci get myapi/items
```

Or set `RSH_PROFILE` in your shell when you want a persistent default.

This pattern keeps one API definition while letting base URLs, auth, headers,
and query defaults vary cleanly by environment.
