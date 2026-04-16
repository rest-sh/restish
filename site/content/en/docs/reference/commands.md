---
title: Commands
linkTitle: Commands
weight: 10
description: Reference index for Restish top-level commands, generated API commands, and plugin-backed command surfaces.
---

Restish has three major command styles:

- generic HTTP commands such as `get`, `post`, `put`, `patch`, and `delete`
- API-aware commands that appear under a configured API name
- plugin-backed commands such as `bulk` and `mcp`

## Path

`Documentation -> Reference -> Commands`

## Top-Level Built-In Commands

These commands are part of the main Restish binary:

- `get`, `post`, `put`, `patch`, `delete`: generic HTTP requests
- `edit`: fetch-edit-update workflow for writable resources
- `links`: fetch a resource and print normalized hypermedia links
- `cert`: inspect a server TLS certificate chain
- `auth-header`: print the resolved `Authorization` header for a configured API
- `api`: manage API registrations and cached specs
- `cache`: inspect or clear the HTTP response cache
- `setup`: configure shell `noglob`-style behavior
- `plugin`: inspect, install, remove, or debug plugins

Dedicated references:

- [Generic HTTP Commands](../reference/http-commands/)
- [Edit Command](../reference/edit-command/)
- [Links Command](../reference/links-command/)
- [Cache Commands](../reference/cache-command/)
- [Cert Command](../reference/cert-command/)
- [Auth Header Command](../reference/auth-header-command/)
- [Setup Command](../reference/setup-command/)
- [Plugin Commands](../reference/plugin-command/)

## Generic HTTP Commands

Use these when you want to make a direct request quickly:

```bash
restish https://api.rest.sh/
restish post https://api.rest.sh name: daniel active: true
```

These commands work without any API registration step.

## API Management Commands

Use `api` commands to register, inspect, and manage APIs described by OpenAPI
or other supported loaders.

Common workflow:

```bash
restish api configure example https://api.rest.sh
restish api list
restish example --help
```

After configuration, Restish generates subcommands under the API name.

## Generated API Commands

After an API is configured, its short name becomes a top-level command group:

```bash
restish api configure example https://api.rest.sh
restish example --help
restish example list-images
```

Generated commands are built from the cached API spec at startup, which keeps
help and completion available without a live network lookup every time.

## Plugin Commands

Plugins can contribute new top-level command surfaces. For example, the MCP
plugin adds `restish mcp ...`, and the bulk plugin adds `restish bulk ...`.

See the [plugin quickstart](/docs/plugins/quickstart/) and
[plugin reference](/docs/reference/plugins/) for the extension model.

## High-Value Commands To Learn Early

- `restish https://api.rest.sh/`
- `restish api configure example https://api.rest.sh`
- `restish example --help`
- `restish auth-header example`
- `restish cache info`
- `restish links https://api.rest.sh/images`
- `restish edit https://api.rest.sh/types`

## Common Global Behavior

Most commands participate in the same shared runtime:

- profiles can inject base URLs, auth settings, TLS options, and defaults
- output can be reformatted with `-o`
- filters can project or transform structured responses
- retries, caching, and pagination apply where relevant
- plugins can intercept requests, responses, auth, loading, and formatting

## Finding Detailed Help

- Run `restish --help` for top-level command discovery.
- Run `restish <command> --help` for command-specific flags and examples.
- Run `restish <api-name> --help` after configuring an API to discover
  generated operations.
- Use the [guides](/docs/guides/) for workflows and the
  [reference section](/docs/reference/) for factual details.
