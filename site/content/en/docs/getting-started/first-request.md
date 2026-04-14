---
title: First Request
linkTitle: First Request
weight: 30
description: Make your first successful request with Restish using a generic HTTP command.
---

The fastest way to learn Restish is to use a generic HTTP request before moving
on to API-specific commands.

## Make A Request

```bash
restish get https://api.rest.sh/
```

You should see a formatted response body on stdout.

If you already installed the binary with Homebrew, this is the quickest
possible end-to-end success check.

## Use The Bare-URL Shortcut

You can also rely on the bare-URL shortcut. In practice, most docs use this
shorter form for plain GET requests, while `get` is still available when you
want to spell the verb out explicitly:

```bash
restish https://api.rest.sh/
```

## What Happened

- `get` is a generic HTTP verb command
- Restish made the request and normalized the response
- the default formatter chose a human-readable output style

## Add One Header

```bash
restish get -H 'Accept: application/json' https://api.rest.sh/images
```

This shows the basic request-building model: target plus optional flags.

## Send A Small Request Body

Post a small structured body:

```bash
restish post https://api.rest.sh name: Alice active: true
```

That shorthand body syntax is one of the fastest ways to use Restish
interactively.

## Know What You Just Learned

After these three examples, you already know the core Restish loop:

- choose a URL or API-relative target
- add flags for headers, query params, output, or auth as needed
- let Restish decode, normalize, and format the response

## What To Try Next

- [Shell Setup](../shell-setup/) for completions and safer shell input
- [Connect to an API](../connect-to-an-api/) for generated commands
- [Requests](../guides/requests/) for the broader workflow guide
- [Example API](/docs/reference/example-api/) for the canonical docs examples
