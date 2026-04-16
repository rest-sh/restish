---
title: First Request
linkTitle: First Request
weight: 30
description: Make your first successful request with Restish and understand what the default output is showing you.
---

The fastest way to learn Restish is to use a generic HTTP request before moving
on to API-specific commands.

## Make A Request

```bash
restish get https://api.rest.sh/
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

If you already installed the binary with Homebrew, this is the quickest
possible end-to-end success check.

## Use The Bare-URL Shortcut

You can also rely on the bare-URL shortcut. In practice, most docs use this
shorter form for plain GET requests, while `get` is still available when you
want to spell the verb out explicitly:

```bash
restish https://api.rest.sh/
```

That works because a bare URL is treated as a generic `GET` request.

## What Happened

- `get` is a generic HTTP verb command
- Restish made the request and normalized the response
- the default formatter chose a human-readable output style

On a terminal, that default format is `readable`, which shows useful HTTP
context plus a structured body. If you redirect output to a file or pipe,
Restish defaults to JSON for structured responses instead.

## Add One Header

```bash
restish get -H 'Accept: application/json' https://api.rest.sh/images
```

Example output:

```readable
HTTP/2.0 200 OK
Content-Type: application/json

[
  {
    format: "jpeg"
    name: "Dragonfly macro"
    self: "/images/jpeg"
  }
  ...
]
```

This shows the basic request-building model: target plus optional flags.

## Send A Small Request Body

Post a small structured body:

```bash
restish post https://api.rest.sh name: Alice active: true
```

That shorthand body syntax is one of the fastest ways to use Restish
interactively.

Conceptually, Restish builds a request body like:

```json
{
  "name": "Alice",
  "active": true
}
```

For small exploratory requests, this is usually faster than writing a separate
JSON file first.

## See The Machine-Friendly Output

If you want the decoded body as JSON instead of the terminal-oriented readable
view, ask for it explicitly:

```bash
restish https://api.rest.sh/images -o json
```

Example output:

```json
[
  {
    "format": "jpeg",
    "name": "Dragonfly macro",
    "self": "/images/jpeg"
  }
]
```

## Know What You Just Learned

After these three examples, you already know the core Restish loop:

- choose a URL or API-relative target
- add flags for headers, query params, output, or auth as needed
- let Restish decode, normalize, and format the response

## What To Try Next

- [Quickstart](../quickstart/) for the full ten-minute path
- [Shell Setup](../shell-setup/) for completions and safer shell input
- [Connect to an API](../connect-to-an-api/) for generated commands
- [Requests](../guides/requests/) for the broader workflow guide
- [Example API](/docs/reference/example-api/) for the canonical docs examples
