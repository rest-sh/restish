---
title: Quickstart
linkTitle: Quickstart
weight: 10
description: Install Restish, make your first request, register the example API, and filter useful output.
aliases:
  - /docs/getting-started/first-request/
---

This tutorial is the on-rails path for new users. You will verify the binary,
make a generic request, send a small body, register the live example API, use a
generated command, and filter output for shell use.

## Prerequisites

You need a `restish` binary on your `PATH`.

Before the first public v2 release, the most reliable path is a source build
from this repository:

```bash
go install ./cmd/restish
restish --help
```

If you installed a release package, verify the binary instead:

```bash
restish --help
```

See [Install](../install/) for the full list of install options.

## 1. Make A Direct Request

Start with a full URL. No config is required.

{{< restish-example >}}
restish api.rest.sh/uuid
{{< /restish-example >}}

In an interactive terminal, Restish renders structured responses in its
readable format by default.

That is generic HTTP mode: Restish sends a request, normalizes the response,
and renders it.

## 2. Spell Out The Verb

A bare URL is a `GET` request:

```bash
restish https://api.rest.sh/
```

This is equivalent to:

```bash
restish get https://api.rest.sh/
```

Use the explicit verb when it makes a command easier to read in scripts.

## 3. Inspect A Header

Use the header fixture when you want to see what the server received:

{{< restish-example >}}
restish -H 'X-Demo: quickstart' https://api.rest.sh/headers
{{< /restish-example >}}

The response includes the `X-Demo` header alongside Restish defaults such as
`Accept`, `Accept-Encoding`, and `User-Agent`.

## 4. Send A Small Body

For JSON APIs, Restish shorthand is the quickest way to build a structured
body:

{{< restish-example >}}
restish post https://api.rest.sh/post 'name: Alice, active: true'
{{< /restish-example >}}

The `/post` fixture echoes the parsed body so you can confirm what was sent.

## 5. Register The Example API

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

## 6. Use Generated Commands

Call an operation by name instead of hand-building the URL:

{{< restish-example >}}
restish example list-images -o table --rsh-columns name,format,self
{{< /restish-example >}}

```bash
restish example get-image jpeg > dragonfly.jpg
```

Generated commands are still normal Restish requests. Profiles, auth, TLS,
retries, pagination, filtering, and output formats all still apply.

## 7. Add A Profile

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
          "headers": ["Accept: application/json"],
        },
      },
    },
  },
}
```

Use it with `-p`:

```bash
restish -p json example list-images
```

## 8. Filter For Shell Use

When you only need one field, filter it and use raw output:

{{< restish-example >}}
restish example list-images -f body.self -r
{{< /restish-example >}}

Example output:

```text
/images/jpeg
/images/webp
/images/gif
/images/png
/images/heic
```

## Choose Machine-Friendly Output

TTY output defaults to `readable`, which is the format you will usually use
interactively. Use `-o json` when the next tool expects a JSON document:

{{< restish-example >}}
restish https://api.rest.sh/images -o json
{{< /restish-example >}}

Use `-r` with a scalar filter when shell scripts need plain text:

{{< restish-example >}}
restish https://api.rest.sh/images -f body.self -r
{{< /restish-example >}}

## What You Learned

You used the main Restish workflow:

1. make a generic request for immediate access
2. use shorthand for small request bodies
3. register an API when repeated work is worth naming
4. use generated commands for discoverability and completion
5. move repeated defaults into profiles
6. filter output down to what scripts need

## Next Step

Run [Shell Setup](../shell-setup/) before you use query strings, filters, or
shorthand heavily in an interactive shell.

## Related Pages

- [Connect to an API](../connect-to-an-api/)
- [Set Up Profiles](../set-up-profiles/)
- [Requests](/docs/guides/requests/)
- [Example API](/docs/reference/example-api/)
