---
title: Connect to an API
linkTitle: Connect to an API
weight: 40
description: Register an API, discover its OpenAPI document, and switch from raw URLs to generated commands.
---

Use generic URL requests for exploration. Register an API when you want named
commands, generated help, shell completion, profiles, and auth tied to a stable
short name.

## Register The Example API

```bash
restish api connect example api.rest.sh
```

Restish discovers `https://api.rest.sh/openapi.json`, stores the API config,
and caches the spec so generated commands are available quickly.

The API name becomes a command group. Use a name that starts with a Unicode
letter or number and then uses only letters, numbers, combining marks, `-`, or
`_`. Names cannot collide with built-in commands such as `api`, `get`, or
`post`.

Inspect what was saved:

```bash
restish api inspect example
restish example --help
```

## Use Generated Commands

```bash
restish example list-images
restish example get-image jpeg > dragonfly.jpg
restish example get-status 404 --rsh-ignore-status-code
```

Generated commands are API-relative. The image command above maps to
`GET /images/{type}` without making you type the full URL.

## Generic vs API-Aware

Both commands below call the example API:

```bash
restish api.rest.sh/images/jpeg
restish example/images/jpeg
restish example get-image jpeg
```

Use the generic form for one-off exploration. Use the generated command when
you want discoverable help, completion, profile-aware config, and less URL
assembly.

After the API is synced, both styles participate in completion:

```bash
restish example/<TAB>
restish example get-image <TAB>
```

## Explicit Spec URLs

If discovery is not available, provide the spec location yourself:

```bash
restish api connect example api.rest.sh --spec https://api.rest.sh/openapi.json
restish api sync example
```

Use `api sync` after the server publishes new operations or changes generated
command metadata.

## Operation Base

Some APIs serve operations under a path prefix. Keep `operation_base` path-only:

```bash
restish api set example 'operation_base: /v1'
```

Use `base_url` for scheme and host, and `operation_base` for the operation path
prefix.

## Project Config

Use `.restish.json` when a project should carry shared Restish API setup:

```bash
restish config trust
restish example list-images
```

Restish discovers `.restish.json` in the current directory or a parent, but only
uses it after you trust the file. Trusted project config layers project APIs and
theme over your global config. If you want one command to use a project file as
the complete config source, pass `--rsh-config .restish.json` or set
`RSH_CONFIG`.

Keep committed project config secret-free. Shared OAuth values such as
`client_id`, `audience`, scopes, and endpoint URLs can live there; API key
values, bearer tokens, passwords, and OAuth client secrets should be omitted or
referenced as `env:NAME`.

## Next Step

[Set Up Profiles](../set-up-profiles/) when repeated headers, auth, environment
URLs, or query defaults start making commands noisy.

## Related Pages

- [OpenAPI Reference](/docs/reference/openapi-cli-integration/)
- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [API Management](/docs/reference/api-management/)
- [Example API](/docs/reference/example-api/)
