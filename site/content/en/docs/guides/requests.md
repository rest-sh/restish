---
title: Requests
linkTitle: Requests
weight: 10
description: Build Restish requests with generic HTTP verbs, generated API commands, headers, query params, bodies, and profiles.
---

Restish supports two request styles: generic HTTP requests for immediate access
and API-aware commands generated from an API description for repeated work.

## Start With A Generic Request

{{< restish-example >}}
restish https://api.rest.sh/get
{{< /restish-example >}}

{{< restish-example >}}
restish post https://api.rest.sh/post 'name: Alice, enabled: true'
{{< /restish-example >}}

Use generic requests when you are exploring, debugging, or calling an endpoint
that has no useful spec.

## Add Headers And Query Params

{{< restish-example >}}
restish \
  -H 'Accept: application/json' \
  -H 'X-Demo: requests' \
  -q search=dragonfly \
  https://api.rest.sh/anything/search
{{< /restish-example >}}

The `/anything` fixture echoes method, path, query, headers, raw body, and
parsed body so you can inspect the exact request shape.

Use quoted URLs when you include query strings directly:

{{< restish-example >}}
restish 'https://api.rest.sh/anything/search?search=dragonfly&active=true'
{{< /restish-example >}}

## Send Request Bodies

For small structured bodies, use shorthand:

{{< restish-example >}}
restish post https://api.rest.sh/post 'name: Alice, tags[]: docs, tags[]: cli'
{{< /restish-example >}}

{{< restish-example >}}
restish -c form post https://api.rest.sh/post 'username: alice, password: secret'
{{< /restish-example >}}

For generated or larger bodies, pipe stdin:

```bash
echo '{"name":"Alice","role":"user"}' | restish post https://api.rest.sh/post
```

Piped structured input can be patched by shorthand arguments:

```bash
echo '{"name":"Alice","role":"user"}' | restish post https://api.rest.sh/post role: admin
```

## Use API-Aware Commands

Register an API when repeated work deserves generated help and completion:

```bash
restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'
restish example list-images
restish example get-image jpeg > dragonfly.jpg
```

Generated commands still support normal request flags:

{{< restish-example >}}
restish example list-images -f body.self -o lines
{{< /restish-example >}}

## Override The Server Temporarily

Use `--rsh-server` when a generated command should hit a different host for one
invocation:

```bash
restish --rsh-server https://api.rest.sh example list-images
```

If you keep using the override, create a profile instead.

## Debug A Request

{{< restish-example >}}
restish --rsh-ignore-status-code https://api.rest.sh/status/404
{{< /restish-example >}}

Verbose output goes to stderr so stdout can remain useful for response data.
Use `/anything` or `/headers` when you need the server to echo what it received.

## Related Pages

- [Input and Shorthand](../input/)
- [Authentication](../authentication/)
- [Profiles](/docs/reference/profiles/)
- [HTTP Commands](/docs/reference/http-commands/)
- [Example API](/docs/reference/example-api/)
