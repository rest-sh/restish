---
title: Quickstart
linkTitle: Quickstart
weight: 15
description: Go from install to a generated API command and filtered output in about ten minutes.
---

This page is the shortest realistic path from "I just installed Restish" to
"I understand why I would keep using it".

The examples use the public docs API at `https://api.rest.sh`.

## 1. Install

Install with Homebrew:

```bash
brew install restish
```

Verify the binary:

```bash
restish --help
```

## 2. Make One Direct Request

Start with a plain GET against a real endpoint:

```bash
restish https://api.rest.sh/
```

Example output:

```readable
HTTP/2.0 200 OK
Content-Type: application/cbor

{
  message: "Welcome to the Restish example API"
  self: "https://api.rest.sh/"
}
```

That is Restish in generic HTTP mode. No API registration is required.

## 3. Register the API

Now give the same API a short name:

```bash
restish api configure example https://api.rest.sh
```

Check what Restish learned:

```bash
restish api list
restish example --help
```

You should now see `example` as a generated command group.

## 4. Use a Generated Command

Call a generated operation:

```bash
restish example list-images
restish example get-image jpeg
```

This is the core product shift:

- generic requests are fast for ad hoc exploration
- generated API commands are better for repeated use

## 5. Add a Profile

When you are repeating environment details, move them into a profile instead of
typing them every time:

```bash
restish api edit
```

Add a profile under `example` such as:

```jsonc
{
  "apis": {
    "example": {
      "base_url": "https://api.rest.sh",
      "profiles": {
        "debug": {
          "headers": ["Accept: application/json"]
        }
      }
    }
  }
}
```

Use it:

```bash
restish -p debug example list-images
```

## 6. Filter the Result

Once the response is larger than you need, filter it:

```bash
restish example list-images -f 'body.self' -r
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

After those six steps, you have already used the most important Restish
workflow:

1. make a quick direct request
2. register an API when it is worth naming
3. switch to generated commands
4. move repeated config into profiles
5. filter output down to what you actually need

## Where To Go Next

- [Install](../install/) for more install methods
- [First Request](../first-request/) for a slower walkthrough
- [Connect to an API](../connect-to-an-api/) for discovery details
- [Set Up Profiles](../set-up-profiles/) for environment-specific config
- [Requests](/docs/guides/requests/) for the broader day-to-day workflow
