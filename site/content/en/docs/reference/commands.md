---
title: Commands
linkTitle: Commands
weight: 10
description: Reference index for Restish built-in commands, generic HTTP verbs, generated API commands, and plugin commands.
aliases:
  - /docs/reference/auth-header-command/
  - /docs/reference/api-auth-inspect/
---

Restish commands fall into a few groups: generic HTTP requests, API management,
configuration and setup, utilities, generated API commands, and plugin
commands. Run any command with `--help` for exact flags and operational notes.

## Generic HTTP Commands

Use generic HTTP commands for one-off requests or APIs without a useful
OpenAPI description.

```bash
restish api.rest.sh/
restish get api.rest.sh/get
restish post api.rest.sh/post 'name: Alice'
restish patch api.rest.sh/patch 'enabled: false'
```

A bare URL without a body sends `GET`. A bare URL with shorthand or stdin body
input sends `POST`.

See [HTTP Commands](../http-commands/) for method inference, request body
rules, output behavior, and HTTP-status exit codes.

## API Management

Use `api` commands when an API has OpenAPI and repeated work deserves
generated commands, profiles, auth setup, and cached specs.

```bash
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
restish api list
restish api sync example
restish api auth inspect example --redact
```

See [API Management](../api-management/) for registration, spec discovery,
sync, config patches, and API auth inspection.

## Configuration And Setup

Use config and setup commands to manage local Restish state and shell
integration.

```bash
restish config show
restish config set 'apis.example.profiles.default.headers[]: "X-Debug: true"'
restish shell setup zsh --dry-run
restish cache info
```

Use [Config](../config/) and [Profiles](../profiles/) for persistent settings,
see [Config Command](../config-command/) and [Cache Command](../cache-command/)
for exact command syntax, and use [Shell Command](../shell-command/) before
heavy use of filters, query strings, or shorthand in interactive shells.

## Utilities

Utilities help inspect TLS, links, content types, runtime health, and editable
resources.

```bash
restish cert api.rest.sh
restish links api.rest.sh/images next
restish content-types
restish doctor -o json
restish edit api.rest.sh/types
```

See [Edit](../edit-command/) for fetch-edit-update workflows and
[Doctor](../doctor-command/) for diagnostics. Smaller tools are covered by
[Utility Commands](../utility-commands/).

## Generated API Commands

After `api connect`, the API name becomes a command group backed by the cached
OpenAPI spec:

```bash
restish example --help
restish example list-images
restish example get-image jpeg
```

Generated commands still use the same request, auth, TLS, pagination,
filtering, and output flags as generic requests.

## Plugin Commands

Command plugins can add top-level commands such as `bulk` and `mcp`.

```bash
restish plugin list
restish plugin install rest-sh/restish csv
restish bulk status
restish mcp serve example
```

See [Plugin Command](../plugin-command/) for install/list/remove/debug, and
[Install and Use Plugins](/docs/plugins/install-and-use/) for operator setup.

## Related Pages

- [HTTP Commands](../http-commands/)
- [API Management](../api-management/)
- [Config Command](../config-command/)
- [Cache Command](../cache-command/)
- [Doctor](../doctor-command/)
- [Shell Command](../shell-command/)
- [Utility Commands](../utility-commands/)
- [Global Flags](../global-flags/)
- [Plugin Command](../plugin-command/)
