---
title: OpenAPI and CLI Integration
linkTitle: OpenAPI Integration
weight: 42
description: Help API authors shape a better Restish CLI experience from their OpenAPI document.
---

Restish works best with OpenAPI documents that are written with CLI use in
mind, not only browser docs or SDK generation in mind.

This page is for API authors who want their API to feel good in Restish once a
user runs:

```bash
restish api configure example https://api.rest.sh
```

## Start With Discoverability

Restish first tries to discover an API description from the base URL.

The most useful setup is a base response that advertises your OpenAPI document
through one of these link relations:

- `service-desc`
- `describedby`

Example:

```http
HTTP/2 204 No Content
Link: </openapi.json>; rel="service-desc"
```

If you do not expose those links, Restish falls back to common paths such as
`/openapi.json` and `/openapi.yaml`.

## How Operations Become Commands

Generated commands roughly look like:

```bash
restish myapi my-operation --optional-flag value required-arg
```

The important mapping rules are:

- the API short name comes from the user's local registration
- command names usually come from `operationId`
- required path and query parameters become positional arguments
- optional parameters become flags
- request bodies still use shorthand or stdin

That makes `operationId` one of the most important author-controlled inputs to
the final CLI shape.

## CLI-Specific OpenAPI Extensions

Restish recognizes these OpenAPI extensions:

- `x-cli-name`
- `x-cli-description`
- `x-cli-aliases`
- `x-cli-hidden`
- `x-cli-ignore`
- `x-cli-config`

## `x-cli-name`

Use this when the best CLI name differs from the best OpenAPI identifier.

```yaml
paths:
  /pets/{petId}:
    get:
      operationId: getPet
      x-cli-name: pet
```

That can produce a friendlier command shape such as:

```bash
restish myapi pet 123
```

## `x-cli-description`

Use this when the CLI help text should emphasize flags and arguments rather
than HTTP transport details.

```yaml
paths:
  /items:
    get:
      description: Returns all items.
      x-cli-description: List items, optionally filtering by status.
```

## `x-cli-aliases`

Use aliases for common short forms:

```yaml
paths:
  /items:
    get:
      operationId: listItems
      x-cli-aliases:
        - ls
```

## `x-cli-hidden` and `x-cli-ignore`

Use `x-cli-hidden` when the operation should still exist but stay out of the
normal help listing.

Use `x-cli-ignore` when the operation, path, or parameter should not become CLI
surface at all.

```yaml
paths:
  /internal:
    x-cli-hidden: true
  /legacy:
    x-cli-ignore: true
```

## `x-cli-config`

`x-cli-config` helps Restish pre-populate a useful local config during
`api configure`.

This is the main way to teach Restish about:

- the preferred auth scheme
- persistent headers
- prompts for usernames or other non-secret values
- parameter templates derived from prompted values

Example:

```yaml
components:
  securitySchemes:
    default:
      type: http
      scheme: basic
x-cli-config:
  security: default
  headers:
    accept: application/json
  prompt:
    username:
      description: Username for the API
      example: alice
    password:
      description: Password for the API
      example: secret
```

Do not put secrets in the OpenAPI document. Use prompts or auth flows that let
the user provide them at configuration or request time.

## Authoring Advice That Pays Off

- Give every user-facing operation a stable `operationId`.
- Prefer operation names that read well in a shell.
- Keep required parameters minimal and clearly named.
- Use enums when possible so completion can surface the valid values.
- Expose your spec through `service-desc` or `describedby`.
- Use `x-cli-config` when the API has a standard auth shape you can teach to
  clients up front.

## Related Pages

- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [API Commands](/docs/concepts/api-commands/)
- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
