---
title: API Management
linkTitle: API Management
weight: 13
description: Reference for registering APIs, syncing specs, editing config, and inspecting API state.
---

`restish api` manages configured APIs and generated command sources.

## Connect

```bash
restish api connect example https://api.rest.sh 'prompt.api_key: docs-key'
```

Discovers a spec, builds initial config, prompts for setup where needed, and
saves the API.

Use `--no-discover` when you only want to save a local base URL without network
spec discovery:

```bash
restish api connect example https://api.rest.sh --no-discover
```

## Explicit Spec

```bash
restish api connect example https://api.rest.sh --spec https://api.rest.sh/openapi.json
```

Uses the provided OpenAPI file or URL instead of discovery. Rerun
`api connect` to refresh generated/default material; pass `--replace` when you
want the rerun to replace generated profile defaults instead of preserving local
profile edits.

## Sync

```bash
restish api sync example
```

Forces a spec refresh after the API publishes changes.

## List And Show

```bash
restish api list
restish api show example
```

## Set And Edit

```bash
restish api set example command_layout: tags
restish api set example operation_base: /v1
restish api edit
```

## Remove

```bash
restish api remove example
```

Removes a configured API. It does not delete remote resources.

## Clear Auth Cache

```bash
restish api clear-auth-cache example
```

Use after OAuth credentials or token state need a fresh flow.

## Auth

```bash
restish api auth list example
restish api auth add example PartnerKey
restish api auth remove example PartnerKey
restish api auth inspect example
restish api auth inspect example --rsh-credential PartnerKey
restish api auth inspect example --raw-header Authorization
```

`api auth` manages profile credential bindings for generated OpenAPI
operations. `inspect` replaces the old top-level auth helper and
also works for non-Authorization credentials such as API-key headers.

## Content Types

```bash
restish api content-types
```

Prints registered input and output content types.

## Related Pages

- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [Config](../config/)
- [Commands](../commands/)
