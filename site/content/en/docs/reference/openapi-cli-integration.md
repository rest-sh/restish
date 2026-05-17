---
title: OpenAPI Extensions and Generated Commands
linkTitle: OpenAPI
weight: 16
description: Reference for shaping generated Restish commands from OpenAPI documents.
aliases:
  - /docs/guides/openapi-cli-integration/
---

Restish turns OpenAPI operations into CLI commands. This reference is for API
authors and maintainers who want the generated command surface to feel natural:
stable names, useful help, predictable parameters, auth setup, and documented
Restish extensions.

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
restish api connect example api.rest.sh 'prompt.api_key: docs-key'
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

Path-level parameters are merged into each operation under that path, and an
operation-level parameter with the same `in` and `name` overrides the path-level
one. Parameters with the same name but different locations, such as a path
`id` and a query `id`, stay separate. Parameters without a schema are treated
as strings.

The generated flag name may be normalized for the shell, but Restish preserves
the original OpenAPI parameter name on the wire. That matters for parameters
such as `$select`, `$filter`, dotted vendor names, and case-sensitive headers.

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

For query parameters with `allowReserved: true`, literal reserved URL
characters are kept unescaped in the generated query value where safe. Literal
`+` is still encoded as `%2B`, and literal `%` is encoded as `%25`, so plus
signs and pre-escaped-looking input are not confused with spaces or decoded
twice.

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
`encoding.contentType` per-part metadata, and opaque body media types such as
`application/octet-stream`, XML, and NDJSON send raw string or file input:

```bash
restish myapi upload-item name: alice, file: @photo.jpg
restish myapi put-blob @payload.bin
restish myapi webdav-operation @propfind.xml
restish myapi insert-json-line @events.ndjson
```

Generated help prefers `@file` for whole-body file input because it keeps the
body source attached to the command invocation. Redirected stdin works too, but
`@file` is easier to copy, review, and combine with required operation
arguments and flags.

OpenAPI allows request bodies on methods such as `GET` and `DELETE`. Restish
will send them when the spec defines a request body and the user supplies body
arguments, but many servers, proxies, and caches treat those requests
inconsistently. Prefer body-bearing methods such as `POST`, `PUT`, or `PATCH`
when you control the API design.

## Schemas And Generated Bodies

Restish uses OpenAPI schemas for help, completions, and example generation.
Generated command request bodies use the same shorthand semantics as generic
HTTP requests. For example, `id: 123` sends a number and `id: "123"` sends a
string, even when the OpenAPI schema says the field is a string.

Schemas are not full request validators by default. Unknown body fields are
allowed unless Restish grows an explicit validation mode. Schema constructs
such as `oneOf`, `anyOf`, `allOf`, `nullable`, `enum`, `const`, defaults,
examples, numeric constraints, read-only/write-only fields, additional
properties, and recursive references are used for help and bounded example
bodies.

Use `--rsh-generate-body` on a generated command to print an example body and
exit:

```bash
restish myapi create-item --rsh-generate-body
```

Generated operation help also shows response media types and response header
names when the spec defines them. That makes pagination headers, rate-limit
headers, and empty-body responses visible without reading the raw OpenAPI
document.

## Server URLs

Restish honors OpenAPI `servers` at document, path, and operation level.
Operation-level servers win over path-level servers, which win over
document-level servers. Server variables use local `server_variables` config
values when provided, then OpenAPI defaults:

```jsonc
{
  "apis": {
    "myapi": {
      "base_url": "https://api.vendor.test/root",
      "server_variables": {
        "version": "v2"
      }
    }
  }
}
```

Relative server URLs resolve against `base_url`. A server such as `v2` with the
base URL above sends generated operation requests under
`https://api.vendor.test/root/v2`.

If a configured server variable value is outside the OpenAPI `enum`, Restish
warns and uses the configured value. Local config represents operator intent,
and specs are sometimes stale. If a configured variable is not declared by any
applicable OpenAPI server and the spec does declare server variables, Restish
fails because the value cannot affect URL expansion.

Absolute server URLs on another origin are blocked unless the API config lists
the origin in `allowed_operation_origins`. `restish api connect --yes` can add
detected safe entries, and interactive `api connect` asks before saving them.
Without that opt-in, generated commands fail with an `allowed_operation_origins`
hint instead of silently routing the request to `base_url`.

Same-origin absolute server URLs are used when scheme, hostname, and effective
port all match.
If a same-origin server points outside the configured base path, Restish keeps
the selected API profile and resolves the generated operation to that path. If
no OpenAPI server matches the configured origin, Restish falls back to
`base_url` instead of sending configured credentials to another host.

Use `operation_base` when the API registration needs an explicit request-path
override independent of the OpenAPI document:

```jsonc
{
  "apis": {
    "myapi": {
      "base_url": "https://api.vendor.test/root",
      "operation_base": "/"
    }
  }
}
```

With that config, generated operation paths are resolved from
`https://api.vendor.test/`.

## External References

Restish resolves external OpenAPI `$ref` documents during `api sync` and local
spec loading. Local `spec_files` may reference nearby files or full `file://`
URIs. Downloaded specs may reference same-origin HTTP(S) documents. Cross-origin
remote refs are blocked unless cross-origin spec discovery is explicitly
enabled and the target passes Restish's trust checks.

Generated operation metadata is cached after sync, so generated commands can
start from the operation cache without refetching secondary reference files.

## Auth Setup Hints

Prefer standard OpenAPI security schemes first. Restish derives basic auth,
API keys, and supported OAuth setup from the spec. Use `x-cli-config` only for
Restish-specific prompting and defaults.

Generated commands honor operation-level security:

- `security: []` is public and sends no configured auth.
- A single effective requirement can use profile-level `auth` for compatibility.
- Multiple alternatives or combined requirements use
  `profiles.<name>.credentials.<credential-id>` bindings.
- `--rsh-auth PartnerKey` or `--rsh-auth UserOAuth+PartnerKey` selects
  one allowed alternative for an operation.

OpenAPI scope and role arrays are matched against credential `satisfies` values.
When a required binding is missing, Restish fails before sending the request.

Never put secrets in the OpenAPI document. Use prompts, environment references,
or external tools.

## Command Layout

Flat layout is the default. Restish does not guess an automatic layout from the
spec. Tag layout can help large APIs when the tags are stable and useful:

```bash
restish api set myapi 'command_layout: tags'
```

Keep tags short and user-facing if you expect them to become command groups.

## Related Pages

- [Connect to an API](/docs/getting-started/connect-to-an-api/)
- [API Setup and Discovery](/docs/guides/api-setup-and-discovery/)
- [API Management](/docs/reference/api-management/)
- [MCP](/docs/plugins/mcp/)
