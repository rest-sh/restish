---
title: Use External-Tool Auth
linkTitle: External Tool Auth
weight: 74
description: Delegate request auth to a local helper script instead of storing secrets directly in Restish config.
---

Example config:

```jsonc
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com",
      "profiles": {
        "default": {
          "auth": {
            "type": "external-tool",
            "params": {
              "commandline": "./scripts/sign-request.sh",
              "omitbody": "true"
            }
          }
        }
      }
    }
  }
}
```

Then make requests normally:

```bash
restish myapi/items
```

The default protocol sends request metadata as JSON on stdin and expects JSON
request updates on stdout:

```json
{"headers":{"Authorization":["Bearer token-from-helper"]}}
```

For helpers that only print a token, use bearer-token output mode:

```jsonc
{
  "type": "external-tool",
  "params": {
    "commandline": "./scripts/token.sh",
    "output": "bearer-token"
  }
}
```

Restish trims stdout and sends `Authorization: Bearer <token>`.

The first time a configured command runs, Restish asks you to approve it and
stores the approved command hash next to the config. Tool stderr is still shown
to you; if the tool fails, Restish includes only a bounded, redacted stderr
excerpt in the returned error.

Remote OpenAPI specs cannot approve or install these commands for you. Treat
the `commandline` value like local executable code: configure it from a trusted
setup script or by editing your own Restish config, then approve it on first
use.
