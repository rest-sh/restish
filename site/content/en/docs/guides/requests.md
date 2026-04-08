---
title: Requests
linkTitle: Requests
weight: 10
description: Learn the core request workflow in Restish, from simple verbs to richer API-aware commands.
---

# Requests

Restish supports both generic HTTP verbs and API-generated commands.

## Start Simple With Generic Requests

Generic requests are best when you want to move quickly:

```bash
restish get https://httpbin.org/json
restish post https://httpbin.org/anything name: daniel active: true
```

You can also omit the verb entirely for a plain GET:

```bash
restish https://httpbin.org/json
```

## Add Headers, Query Params, And Timeouts

The most common request-shaping flags are:

- `-H` or `--rsh-header` for request headers
- `-q` or `--rsh-query` for query parameters
- `--rsh-timeout` for request timeouts
- `-c` or `--rsh-content-type` for request body encoding

Examples:

```bash
restish get \
  -H 'Accept: application/json' \
  -q per_page=100 \
  -q status=active \
  --rsh-timeout 15s \
  https://api.example.com/items
```

## Send Request Bodies

For small structured payloads, shorthand is the fastest path:

```bash
restish post https://api.example.com/users \
  name: Alice \
  role: admin \
  enabled: true
```

Restish turns that into a structured body and encodes it using the selected
content type. JSON is the default.

If you need a different encoding:

```bash
restish post -c yaml https://api.example.com/users name: Alice
```

## Read From Stdin

Restish can also read request data from stdin.

For example:

```bash
echo '{"name":"Alice","role":"admin"}' | \
  restish post https://api.example.com/users
```

You can combine piped input with shorthand patch args. Stdin becomes the base
document, and the CLI args override it:

```bash
echo '{"name":"Alice","role":"user"}' | \
  restish post https://api.example.com/users role: admin
```

## Override The Server Without Rewriting Paths

When you already have an API-aware command but need a different host, use
`--rsh-server`:

```bash
restish --rsh-server https://staging.example.com myapi users list
```

This is useful for temporary environment switching without changing the saved
API config.

## API-Aware Requests

API-aware requests become available when Restish knows about an API
description. They trade a little setup for much richer day-to-day ergonomics.

For example:

```bash
restish api configure petstore https://api.example.com
restish petstore --help
```

At that point, operations from the API description become subcommands under the
API name.

## When API-Aware Commands Are Better

Use generated API commands when you want:

- discoverable operation names
- generated help and shell completion
- stable API-relative paths instead of full URLs
- profile-aware base URLs and auth

Use generic requests when you want:

- a one-off call right now
- to explore an endpoint before formal setup
- to work with an API that does not have a usable description

## Profiles Affect Requests

The active profile can supply:

- base URL overrides
- default headers
- default query params
- auth configuration
- TLS settings such as a signer plugin

Choose the active profile with `-p` or `--rsh-profile`:

```bash
restish -p staging get myapi/items
```

## Debugging A Request

When you need to understand what Restish is sending and receiving:

- use `-v` to print request and response headers
- use `-vv` to include more TLS detail
- use `--rsh-ignore-status-code` when you want output without a failing exit
  status getting in the way

Example:

```bash
restish -v --rsh-ignore-status-code get https://api.example.com/items
```

## Related Guides

- [Input and Shorthand](../input/)
- [Authentication](../authentication/)
- [TLS](../tls/)
- [Pagination and Links](../pagination/)
