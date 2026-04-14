---
title: Requests
linkTitle: Requests
weight: 10
description: Learn the core request workflow in Restish, from simple verbs to richer API-aware commands.
---

Restish supports both generic HTTP verbs and API-generated commands.

## The Core Mental Model

Almost every Restish request follows the same pattern:

1. choose a target, either a full URL or a configured API name
2. add any per-request flags you need
3. let Restish apply profile defaults, auth, TLS, filtering, pagination, and output rules

That is why the tool feels consistent even as your usage gets more advanced.

## Start Simple With Generic Requests

Generic requests are best when you want to move quickly:

```bash
restish https://api.rest.sh/
restish post https://api.rest.sh name: daniel active: true
```

You can also omit the verb entirely for a plain GET:

```bash
restish https://api.rest.sh/
```

This is the right starting point when:

- you are exploring an API for the first time
- you only need one call
- there is no useful API description yet

## Add Headers, Query Params, And Timeouts

The most common request-shaping flags are:

- `-H` or `--rsh-header` for request headers
- `-q` or `--rsh-query` for query parameters
- `--rsh-timeout` for request timeouts
- `-c` or `--rsh-content-type` for request body encoding

Examples:

```bash
restish \
  -H 'Accept: application/json' \
  -q search=macro \
  -q format=jpeg \
  --rsh-timeout 15s \
  https://api.rest.sh/
```

For one-off calls, this is often enough. Restish starts paying off more when
you move repeated headers, auth, and environment details into profiles.

## Send Request Bodies

For small structured payloads, shorthand is the fastest path:

```bash
restish post https://api.rest.sh \
  name: Alice \
  role: admin \
  enabled: true
```

Restish turns that into a structured body and encodes it using the selected
content type. JSON is the default.

If you need a different encoding:

```bash
restish post -c yaml https://api.rest.sh name: Alice
```

For larger payloads or generated data, prefer stdin:

```bash
cat user.json | restish post https://api.rest.sh
```

## Read From Stdin

Restish can also read request data from stdin.

For example:

```bash
echo '{"name":"Alice","role":"admin"}' | \
  restish post https://api.rest.sh
```

You can combine piped input with shorthand patch args. Stdin becomes the base
document, and the CLI args override it:

```bash
echo '{"name":"Alice","role":"user"}' | \
  restish post https://api.rest.sh role: admin
```

That patch-style stdin behavior is one of the most useful small productivity
features in Restish.

## Override The Server Without Rewriting Paths

When you already have an API-aware command but need a different host, use
`--rsh-server`:

```bash
restish --rsh-server https://api.rest.sh example list-images
```

This is useful for temporary environment switching without changing the saved
API config.

Use this as an escape hatch, not a replacement for profiles. If you keep doing
it, that is usually a sign you should add a profile.

## API-Aware Requests

API-aware requests become available when Restish knows about an API
description. They trade a little setup for much richer day-to-day ergonomics.

For example:

```bash
restish api configure example https://api.rest.sh
restish example --help
```

At that point, operations from the API description become subcommands under the
API name.

That changes the daily workflow in a few important ways:

- the CLI becomes more discoverable through help and completion
- required params become arguments or flags instead of hand-built URLs
- API-relative commands are easier to read and share

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

In practice, many users do both:

- generic requests for exploration and odd jobs
- API commands for repeatable operations

## Profiles Affect Requests

The active profile can supply:

- base URL overrides
- default headers
- default query params
- auth configuration
- TLS settings such as a signer plugin

Choose the active profile with `-p` or `--rsh-profile`:

```bash
restish -p debug get example/images
```

If you already know you will hit multiple environments, set profiles up early.
They keep request commands short and keep environment details out of shell
history.

## Debugging A Request

When you need to understand what Restish is sending and receiving:

- use `-v` to print request and response headers
- use `-vv` to include more TLS detail
- use `--rsh-ignore-status-code` when you want output without a failing exit
  status getting in the way

Example:

```bash
restish -v --rsh-ignore-status-code https://api.rest.sh/images
```

## A Good Progression

For most users, the request workflow matures in this order:

1. generic `get` and `post` calls
2. shorthand bodies and stdin
3. profiles for repeated environments and auth
4. API-aware commands from a discovered spec
5. filters, pagination, and output tuning for day-to-day use

## Related Guides

- [Input and Shorthand](../input/)
- [Authentication](../authentication/)
- [Set Up Profiles](/docs/getting-started/set-up-profiles/)
- [TLS](../tls/)
- [Pagination and Links](../pagination/)
