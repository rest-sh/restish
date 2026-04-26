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
- `anyOf` and `oneOf` parameters that do not have one clear scalar type become
  string flags, so they remain usable from the CLI

That makes `operationId` one of the most important author-controlled inputs to
the final CLI shape.

Generated commands are operation-first. Generic HTTP commands are
method-and-path-first. For example:

```bash
restish example create-repo org: myorg repo: myrepo
restish put example/repos org: myorg repo: myrepo
```

The first form comes from an OpenAPI operation. The second form remains useful
for exploration or for paths that are not represented in the spec yet.

Generated operation help is operation-focused by default. It hides inherited
global Restish flags so the operation arguments, flags, schemas, and examples
are easier to scan. Users can run the same operation with `--help-all` to see
the full inherited flag set.

The default generated command layout is flat. Operators can opt into first-tag
subcommands with local config when that makes a large API easier to navigate:

```bash
restish api set example command_layout: tags
restish example repos create-repo org: myorg repo: myrepo
```

For operations with request bodies, users can ask Restish to print an example
body without sending a request:

```bash
restish myapi create-item --rsh-generate-body
```

The example comes from OpenAPI examples, schema examples/defaults/enums, or
schema-derived placeholders. Users can redirect it to a file, edit it, and pass
that file back as request input.

## Query Parameter Serialization

Scalar query parameters are single-value flags or positional arguments:

```yaml
parameters:
  - name: filter
    in: query
    schema:
      type: string
```

```bash
restish example list-items --filter active
# sends ?filter=active
```

Repeated query parameters should be modeled as OpenAPI arrays. Restish treats
scalar params as single values; they are intentionally not repeatable unless
the spec says the parameter is an array.

For the common repeated form, use `style: form` and `explode: true`:

```yaml
parameters:
  - name: tag
    in: query
    style: form
    explode: true
    schema:
      type: array
      items:
        type: string
```

```bash
restish example list-items --tag red --tag blue
# sends ?tag=red&tag=blue
```

For comma-joined arrays, use `style: form` and `explode: false`:

```yaml
parameters:
  - name: ids
    in: query
    style: form
    explode: false
    schema:
      type: array
      items:
        type: string
```

```bash
restish example list-items --ids 10 --ids 20
# sends ?ids=10,20
```

## CLI-Specific OpenAPI Extensions

Restish recognizes these OpenAPI extensions:

- `x-cli-name`
- `x-cli-description`
- `x-cli-aliases`
- `x-cli-hidden`
- `x-cli-ignore`
- `x-cli-config`
- `x-mcp-ignore`

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

## `x-mcp-ignore`

Use this to hide an operation from `restish mcp` without hiding it from the
regular CLI:

```yaml
paths:
  /internal/debug:
    get:
      operationId: debugInternal
      x-mcp-ignore: true
```

This is useful when an operation is valid for direct CLI use but is not safe or
useful to expose as an AI tool.

Use `x-cli-ignore` when you want the operation excluded from both the CLI and
MCP tools.

## `x-cli-config`

`x-cli-config` helps Restish pre-populate a useful local config during
`api configure`.

This is the main way to teach Restish about:

- the preferred auth scheme
- persistent headers
- profile-specific base URLs, headers, query params, and auth params

If `x-cli-config` is absent, Restish can still infer useful profile auth from
OpenAPI security schemes where the mapping is unambiguous.

Example:

```yaml
components:
  securitySchemes:
    default:
      type: http
      scheme: basic
x-cli-config:
  profiles:
    default:
      headers:
        - "Accept: application/json"
      auth:
        type: http-basic
        params:
          username: alice
```

Do not put secrets in the OpenAPI document. v1-style `x-cli-config.prompt` is
still supported for specs already in the wild: `api configure` prompts once,
writes the answers into the local profile config, and never prompts from
`x-cli-config` during normal requests.

Remote specs also cannot install or approve `external-tool` auth commands.
Use `x-cli-config` to describe the desired auth profile, then have users or a
trusted setup script configure the local command line explicitly.

Legacy prompt-shaped config is normalized to the `default` profile:

```yaml
x-cli-config:
  security: default
  prompt:
    client_id:
      description: Client identifier
      example: abc123
    org:
      description: Organization ID
      exclude: true
  params:
    audience: https://example.com/{org}
```

Prompted values are saved as auth params unless `exclude: true` is set. Excluded
values can still be used in `{name}` templates for `params`, headers, and auth
params during `api configure`.

If a required prompt is left blank or an answer does not match its enum, Restish
prints guidance and retries the same prompt. Prompt answers are treated as
literal values, so an answer like `{org}` is not expanded again when headers or
auth params are written.

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
