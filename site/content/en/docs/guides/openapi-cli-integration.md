---
title: OpenAPI and CLI Integration
linkTitle: OpenAPI and CLI Integration
weight: 25
description: Shape a better Restish command surface from an OpenAPI document.
---

Restish turns OpenAPI operations into CLI commands. API authors can make that
surface better with clear operation IDs, useful descriptions, parameter schemas,
security schemes, and a few Restish extensions.

## Minimum Good Operation

```yaml
paths:
  /items/{item-id}:
    get:
      operationId: getItem
      summary: Get one item
      parameters:
        - name: item-id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Item
```

This can become:

```bash
restish myapi get-item alpha
```

## Discoverability

Publish the spec at a predictable location such as `/openapi.json`, or expose a
`service-desc` or `describedby` link from the API root. Verify with:

```bash
restish api configure example https://api.rest.sh 'prompt.api_key: docs-key'
restish example --help
```

## Naming

Restish prefers stable operation IDs. Use extensions when the generated name
would not be good CLI vocabulary:

```yaml
x-cli-name: list-items
x-cli-aliases: [items]
x-cli-description: List items with optional filtering.
```

## Hide Or Ignore Operations

```yaml
x-cli-hidden: true
x-cli-ignore: true
x-mcp-ignore: true
```

Hidden operations remain callable by exact name when supported. Ignored
operations are left out of the generated command surface.

## Query Parameter Serialization

Model repeatable flags as arrays:

```yaml
parameters:
  - name: tag
    in: query
    schema:
      type: array
      items:
        type: string
```

Then users can repeat the flag or pass repeated query params according to the
generated command behavior.

## Auth Setup Hints

Prefer standard OpenAPI security schemes first. Restish can derive basic auth,
bearer auth, API keys, and OAuth setup from the spec. Use `x-cli-config` only
for Restish-specific prompting and defaults.

Never put secrets in the OpenAPI document. Use prompts, environment references,
or external tools.

## Command Layout

Flat layout is easiest for small APIs. Tag layout can help large APIs:

```bash
restish api set myapi command_layout: tags
```

Keep tags short and user-facing if you expect them to become command groups.

## Related Pages

- [API Commands](/docs/concepts/api-commands/)
- [API Setup and Discovery](../api-setup-and-discovery/)
- [API Management](/docs/reference/api-management/)
- [MCP](../mcp/)
