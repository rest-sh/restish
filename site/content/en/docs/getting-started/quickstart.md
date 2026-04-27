---
title: Quickstart
linkTitle: Quickstart
weight: 15
description: Go from a working binary to generated API commands and filtered output in about ten minutes.
---

This tutorial gives you one complete Restish loop: make a generic request,
register the live example API, use generated commands, add a profile, and
filter output for shell use.

## Prerequisites

You need a `restish` binary on your `PATH`. Before the first public v2 release,
the most reliable path is a source build from this repository:

```bash
go build ./cmd/restish
```

If you installed a release package, verify it instead:

```bash
restish --help
```

## 1. Make A Direct Request

Start with a full URL. No config is required.

```bash
restish https://api.rest.sh/
```

In an interactive terminal, Restish renders structured responses in its
readable format by default. You should see an echo response with fields such as:

```readable
method: "GET"
host: "api.rest.sh"
path: "/"
url: "https://api.rest.sh/"
```

That is generic HTTP mode: Restish sends a request, normalizes the response,
and renders it.

## 2. Inspect A Header

Use the header fixture when you want to see what the server received:

```bash
restish -H 'X-Demo: quickstart' https://api.rest.sh/headers
```

The response includes the `X-Demo` header alongside Restish defaults such as
`Accept`, `Accept-Encoding`, and `User-Agent`.

## 3. Register The Example API

Give the same API a short name:

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
```

Check what Restish learned:

```bash
restish example --help
```

You should see generated commands such as `list-images`, `get-image`,
`get-types-example`, and `get-status`.

## 4. Use Generated Commands

Call an operation by name instead of hand-building the URL:

```bash
restish example list-images -o table --rsh-columns name,format,self
restish example get-image jpeg -o raw > dragonfly.jpg
```

Generated commands are still normal Restish requests. Profiles, auth, TLS,
retries, pagination, filtering, and output formats all still apply.

## 5. Add A Profile

Profiles keep repeated request defaults out of every command. Open the config:

```bash
restish api edit
```

Add a profile under the `example` API:

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "profiles": {
        "json": {
          "headers": ["Accept: application/json"]
        }
      }
    }
  }
}
```

Use it with `-p`:

```bash
restish -p json example list-images
```

## 6. Filter For Shell Use

When you only need one field, filter it and use raw output:

```bash
restish example list-images -f body.self -r
```

Example output:

```text
/images/jpeg
/images/webp
/images/gif
/images/png
/images/heic
```

## What You Learned

You used the main Restish workflow:

1. make a generic request for immediate access
2. register an API when repeated work is worth naming
3. use generated commands for discoverability and completion
4. move repeated defaults into profiles
5. filter output down to what scripts need

## Related Pages

- [First Request](../first-request/)
- [Connect to an API](../connect-to-an-api/)
- [Set Up Profiles](../set-up-profiles/)
- [Requests](/docs/guides/requests/)
- [Example API](/docs/reference/example-api/)
