---
title: First Request
linkTitle: First Request
weight: 30
description: Make your first successful request with Restish and understand what the default output is showing you.
---

The fastest way to learn Restish is to make a generic HTTP request before you
configure any API-specific commands.

## Make A Request

```bash
restish https://api.rest.sh/
```

Representative output:

```readable
method: "GET"
host: "api.rest.sh"
path: "/"
url: "https://api.rest.sh/"
```

The real response also includes request headers. The root endpoint is an echo
fixture, so it is useful for seeing what Restish sent.

## Spell Out The Verb

A bare URL is a `GET` request:

```bash
restish https://api.rest.sh/
```

This is equivalent to:

```bash
restish get https://api.rest.sh/
```

Use the explicit verb when it makes a command easier to read in scripts.

## Add One Header

```bash
restish -H 'X-Demo: first-request' https://api.rest.sh/headers
```

Representative output:

```readable
headers: {
  X-Demo: "first-request"
  User-Agent: "restish/2.0.0-dev"
}
```

## Send A Small Body

For JSON APIs, Restish shorthand is the quickest way to build a structured
body:

```bash
restish post https://api.rest.sh/post 'name: Alice, active: true'
```

The `/post` fixture echoes the parsed body so you can confirm what was sent.

## Choose Machine-Friendly Output

TTY output defaults to `readable`, which is the format you will usually use
interactively. Use `-o json` only when the next tool expects JSON:

```bash
restish https://api.rest.sh/images -o json
```

Use `-r` with a scalar filter when shell scripts need plain text:

```bash
restish https://api.rest.sh/images -f body.self -r
```

## What Happened

- Restish treated the URL as a generic HTTP request.
- It applied default `Accept`, `Accept-Encoding`, retry, cache, and output behavior.
- It normalized the response into roots such as `headers`, `links`, and `body`.
- The selected output format decided what you saw.

## Related Pages

- [Quickstart](../quickstart/)
- [Shell Setup](../shell-setup/)
- [Requests](/docs/guides/requests/)
- [Output](/docs/guides/output/)
- [HTTP Commands](/docs/reference/http-commands/)
