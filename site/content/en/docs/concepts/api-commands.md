---
title: API Commands
linkTitle: API Commands
weight: 30
description: See how Restish turns API descriptions into generated commands.
---

API commands are generated from API descriptions, usually OpenAPI documents.
They replace hand-built URLs with named operations while keeping normal Restish
request behavior.

## Start With A Short Name

```bash
restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'
restish example --help
```

The API name becomes a command group. Operations become subcommands such as
`list-images`, `get-image`, and `get-status`.

## What Changes

A generic request says exactly where to go:

```bash
restish https://api.rest.sh/images/jpeg
```

A generated command says what operation to run:

```bash
restish example get-image jpeg
```

The generated command can provide help, completion, required path parameters,
optional flags, and profile-aware base URL selection.

## What Stays The Same

Generated commands still support the same Restish behavior:

- profiles and auth
- TLS options
- request bodies from shorthand or stdin
- retries and cache
- pagination and streaming
- filtering and output formats

## Command Layout

Restish defaults to a flat command layout. There is no automatic layout mode.
APIs with many operations can opt into tag-based layout:

```bash
restish api set example command_layout: tags
```

API authors can improve the generated surface with `operationId`, tags,
descriptions, parameter schemas, examples, and Restish-specific `x-cli-*`
extensions.

## Related Pages

- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [OpenAPI and CLI Integration](/docs/guides/openapi-cli-integration/)
- [API Management](/docs/reference/api-management/)
- [Commands](/docs/reference/commands/)
