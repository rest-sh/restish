---
title: First Request
linkTitle: First Request
weight: 30
description: Make your first successful request with Restish using a generic HTTP command.
---

# First Request

The fastest way to learn Restish is to use a generic HTTP request before moving
on to API-specific commands.

## Make a Request

```bash
restish get https://httpbin.org/json
```

You should see a formatted response body on stdout.

You can also rely on the bare-URL shortcut:

```bash
restish https://httpbin.org/json
```

## What Happened

- `get` is a generic HTTP verb command
- Restish made the request and normalized the response
- the default formatter chose a human-readable output style

## Try One Small Variation

Add a header:

```bash
restish get -H 'Accept: application/json' https://httpbin.org/json
```

Or post a small structured body:

```bash
restish post https://httpbin.org/anything name: Alice active: true
```

That shorthand body syntax is one of the fastest ways to use Restish
interactively.

## What To Try Next

- [Shell Setup](../shell-setup/) for completions and safer shell input
- [Connect to an API](../connect-to-an-api/) for generated commands
- [Requests](../guides/requests/) for the broader workflow guide
