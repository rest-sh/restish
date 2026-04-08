---
title: Call a JSON API
linkTitle: Call a JSON API
weight: 10
description: A quick recipe for calling a JSON API with Restish.
---

# Call a JSON API

Use a generic request when you just want to talk to an endpoint immediately.

```bash
restish get https://httpbin.org/json
```

If you need a header:

```bash
restish get \
  -H 'Accept: application/json' \
  https://api.example.com/items
```

If you need bearer auth repeatedly, put it in a profile instead of repeating
the header by hand each time.

Example config:

```json
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "headers": ["Accept: application/json"],
          "auth": {
            "type": "http-basic",
            "params": {
              "username": "alice"
            }
          }
        }
      }
    }
  }
}
```

Then call the API with either style:

```bash
restish get myapi/items
restish myapi items list
```
