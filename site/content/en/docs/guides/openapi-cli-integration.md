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

Restish maps required parameters to positional arguments and optional
parameters to flags. Required path parameters appear first, followed by required
query, header, and cookie parameters in spec order:

```yaml
parameters:
  - name: item-id
    in: path
    required: true
    schema:
      type: string
  - name: account
    in: query
    required: true
    schema:
      type: string
  - name: page
    in: query
    schema:
      type: integer
```

This becomes:

```bash
restish myapi get-item alpha acct-123 --page 2
```

Model repeatable optional flags as arrays:

```yaml
parameters:
  - name: tag
    in: query
    schema:
      type: array
      items:
        type: string
```

Then users can repeat the flag:

```bash
restish myapi list-items --tag red --tag sale
```

Restish supports the common OpenAPI parameter styles used by generated
commands:

- query: `form`, `spaceDelimited`, `pipeDelimited`, and `deepObject`
- path: `simple`, `label`, and `matrix`
- header: `simple`
- cookie: `form`
- JSON parameter `content` for `application/json` and `+json` media types

For query parameters with `allowReserved: true`, reserved URL characters are
kept unescaped in the generated query value.

OpenAPI header parameters named `Accept`, `Content-Type`, or `Authorization`
are ignored by generated commands. Configure response negotiation, request body
content type, and authentication through Restish flags, profiles, or OpenAPI
security schemes instead.

Unsupported styles are not silently ignored. Generated command help calls them
out so the API author can choose a supported style or the Restish implementation
can grow deliberately.

## Request Bodies

Generated commands use the normal Restish body syntax. JSON and `+json` media
types use shorthand assignments, form bodies use form encoding, multipart
bodies can include fields, repeated file-array fields, and
`encoding.contentType` per-part metadata, and `application/octet-stream` sends
raw string or file input:

```bash
restish myapi upload-item name: alice, file: @photo.jpg
restish myapi put-blob @payload.bin
```

OpenAPI allows request bodies on methods such as `GET` and `DELETE`. Restish
will send them when the spec defines a request body and the user supplies body
arguments, but many servers, proxies, and caches treat those requests
inconsistently. Prefer body-bearing methods such as `POST`, `PUT`, or `PATCH`
when you control the API design.

## Server URLs

Restish honors OpenAPI `servers` at document, path, and operation level.
Operation-level servers win over path-level servers, which win over
document-level servers. Server variables use local `server_variables` config
values when provided, then OpenAPI defaults:

```jsonc
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com/root",
      "server_variables": {
        "version": "v2"
      }
    }
  }
}
```

Relative server URLs resolve against `base_url`. A server such as `v2` with the
base URL above sends generated operation requests under
`https://api.example.com/root/v2`.

Use `operation_base` when the API registration needs an explicit request-path
override independent of the OpenAPI document:

```jsonc
{
  "apis": {
    "myapi": {
      "base_url": "https://api.example.com/root",
      "operation_base": "/"
    }
  }
}
```

With that config, generated operation paths are resolved from
`https://api.example.com/`.

## External References

Restish resolves external OpenAPI `$ref` documents during `api sync` and local
spec loading. Local `spec_files` may reference nearby files or full `file://`
URIs. Downloaded specs may reference same-origin HTTP(S) documents. Cross-origin
remote refs are blocked unless cross-origin spec discovery is explicitly
enabled and the target passes Restish's trust checks.

Generated operation metadata is cached after sync, so generated commands can
start from the operation cache without refetching secondary reference files.

## Auth Setup Hints

Prefer standard OpenAPI security schemes first. Restish can derive basic auth,
bearer auth, API keys, and OAuth setup from the spec. Use `x-cli-config` only
for Restish-specific prompting and defaults.

An operation with `security: []` is treated as public and generated commands do
not attach configured profile auth. Full per-operation scheme matching for APIs
with multiple auth schemes is planned before v2 release; until then, choose the
profile that matches the operation you are calling.

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
