---
title: API Commands
linkTitle: API Commands
weight: 30
description: See how Restish turns API descriptions into generated commands.
---

API commands are generated from API descriptions, usually OpenAPI documents.

They let Restish expose operations as CLI subcommands instead of requiring
users to type every request manually.

## How They Start

First, you register an API under a short name:

```bash
restish api configure example https://api.rest.sh
```

After Restish discovers or loads a usable spec, that short name becomes a
generated command group:

```bash
restish example --help
```

## How Operations Become CLI Commands

Restish builds generated commands from the API description. In the OpenAPI
case:

- operations are grouped under the API short name
- command names usually come from `operationId`
- required path and query parameters become positional arguments
- optional parameters become flags
- request bodies still use shorthand or stdin like generic commands

That keeps generated commands feeling like native CLI commands instead of a
separate subsystem.

## Example Shape

An operation like:

```yaml
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      x-cli-name: pet
```

can turn into a command like:

```bash
restish example get-image jpeg
```

instead of a manual URL such as:

```bash
restish https://api.rest.sh/images/jpeg
```

## Why Generated Commands Feel Better

They improve day-to-day ergonomics by giving you:

- discoverable subcommands
- generated help text
- shell completion
- API-relative parameters instead of repeated full URLs
- the same profile, auth, TLS, pagination, and output behaviors as generic
  requests

## CLI Hints From The Spec

OpenAPI extensions can shape the generated CLI. Restish recognizes hints such
as:

- `x-cli-name`
- `x-cli-description`
- `x-cli-aliases`
- `x-cli-hidden`
- `x-cli-ignore`

That gives API authors some control over how the generated command surface
reads in a shell.

## Cached Specs Matter

Generated commands are built from cached specs at startup rather than requiring
live network discovery every time you run the CLI.

That makes the command tree more predictable and avoids making normal help or
completion behavior depend on the network.

## Related Guides

- [Connect to an API](../getting-started/connect-to-an-api/)
- [Requests](../guides/requests/)

## Source Material

- [Design Records](/docs/contributing/design-records/)
