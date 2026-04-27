---
title: API Management
linkTitle: API Management
weight: 13
description: Reference for registering APIs, syncing specs, editing config, and inspecting API state.
---

`restish api` manages configured APIs and generated command sources.

## Configure

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
```

Discovers a spec, builds initial config, prompts for setup where needed, and
saves the API.

## Add

```bash
restish api add example https://api.rest.sh spec_url: https://api.rest.sh/openapi.json
```

Adds config quickly and accepts shorthand-style field updates.

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

## Delete

```bash
restish api delete example
```

Removes a configured API. It does not delete remote resources.

## Clear Auth Cache

```bash
restish api clear-auth-cache example
```

Use after OAuth credentials or token state need a fresh flow.

## Content Types

```bash
restish api content-types
```

Prints registered input and output content types.

## Related Pages

- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [Config](../config/)
- [Commands](../commands/)
