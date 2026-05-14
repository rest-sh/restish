# `restish-mcp` Plugin

## Summary

`restish-mcp` exposes one or more registered Restish APIs as MCP tools over
stdio. It is the concrete command-plugin example that validates the generic
command-plugin protocol against a real integration target.

Its job is to translate cached OpenAPI operations into MCP tool definitions,
then delegate actual HTTP execution back to Restish.

The product positioning is deliberately narrow: use Restish itself for precise
human CLI calls, and use `restish-mcp` when an MCP client or agent should see
selected OpenAPI operations as tools.

## Goals

- expose API operations as MCP tools without duplicating Restish's HTTP stack
- make tool naming and filtering predictable
- keep the plugin a protocol bridge rather than a second API client
- preserve Restish auth, TLS, retry, cache, and profile behavior for tool calls
- keep the MCP surface intentionally small and operator-friendly

## Non-Goals

- exposing every possible MCP transport or capability from day one
- deriving tools from APIs that lack a usable operation identity
- inventing a custom non-OpenAPI tool schema unrelated to the underlying API

## Position In The Architecture

This plugin is a concrete implementation of the generic command-plugin design.

It demonstrates all of these host/plugin capabilities at once:

- plugin-contributed top-level command
- delegated spec loading
- delegated HTTP execution
- passthrough stdio for a second protocol layered on top

If this plugin became awkward, that would indicate the generic command-plugin
design was missing an important capability.

## Manifest And Command Shape

The plugin advertises:

- `name: mcp`
- `hooks: ["command"]`

and contributes a single command declaration:

- `mcp`

That command opts into `passthrough_stdio`, which is what allows the plugin to
speak JSON-RPC over stdio to an MCP client while still living inside Restish's
command-plugin transport.

## Nested Protocols

The lifecycle has two nested protocols:

1. Restish and the plugin speak the command-plugin CBOR protocol.
2. The plugin and the MCP client speak JSON-RPC over stdio.

The plugin therefore has to keep those concerns separate:

- use command-plugin messages for host capabilities
- use JSON-RPC for MCP-facing behavior

## Startup Lifecycle

At startup:

1. Restish sends the command-plugin `init` message.
2. The plugin parses command flags and selected API names.
3. The plugin starts bounded passthrough-stdin forwarding immediately, so MCP
   client frames that arrive during spec loading are queued to the server pipe
   instead of an unbounded buffer.
4. The plugin asks Restish for each API spec using `api-spec`.
5. The plugin converts the specs into MCP tool definitions.
6. The plugin starts serving MCP stdio traffic.

This design keeps spec loading and HTTP transport inside Restish while letting
the plugin own the MCP-facing protocol. The plugin should use the public
command-plugin client helpers for host HTTP delegation, API spec fetches,
timeouts, stdout/stderr messages, and passthrough stdio. MCP-specific code owns
MCP JSON-RPC and tool mapping only; it should not carry a second
pending-request protocol implementation.

The accepted service invocation is:

```text
restish mcp serve [flags] <api...>
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--operations <id,id>` | Allowlist operation IDs before tool registration. |
| `--read-only` | Expose only `GET` and `HEAD`, even if write tools are otherwise allowed. |
| `--allow-write-tools` | Expose `POST`, `PUT`, `PATCH`, and `DELETE` operations. |
| `--max-result-bytes <n>` | Truncate MCP text results after this many bytes; default is 16 KiB. |
| `--request-timeout <seconds>` | Per-tool delegated HTTP timeout; default is 60 seconds, `0` disables the local plugin timeout. |

`--http` is intentionally not accepted. The plugin is stdio-first for v2; if an
HTTP transport returns later, it should be designed as a new explicit service
mode rather than kept as a hidden compatibility flag.

## Tool Inclusion Rules

`restish-mcp` only exposes operations that have a stable operation identity.
Today that means an operation must have an `operationId`.

Operations are skipped when:

- `x-cli-ignore` is true
- `x-mcp-ignore` is true
- `--read-only` is set and the method is not `GET` or `HEAD`
- the method is `POST`, `PUT`, `PATCH`, or `DELETE` and
  `--allow-write-tools` is not set
- `--operations` is set and the `operationId` is not allowlisted

This is a product decision, not just a parser shortcut. The plugin wants stable
tool names that API authors can reason about.

MCP is model-facing automation, so it is read-biased by default. Write-like
operations must require an explicit operator choice even when the OpenAPI spec
describes them correctly. Explicit hide metadata such as `x-mcp-ignore` remains
authoritative. If `--read-only` and `--allow-write-tools` are both supplied,
read-only wins; this keeps the safer flag dominant in generated MCP client
configuration.

## Tool Naming

When serving one API, tool names are based directly on `operationId`.

When serving multiple APIs, tool names are namespaced as:

- `<apiName>__<operationId>`

That avoids collisions while keeping single-API use ergonomic.

The separator and naming rule should stay deterministic so operators and MCP
clients can rely on tool identity across runs.

## Tool Schema Generation

The generated MCP input schema is built from:

- OpenAPI parameters mapped by name
- request-body schema exposed as a `body` property when applicable
- the operation summary or description for tool help text

This is intentionally close to the OpenAPI shape rather than a custom MCP-only
abstraction.

The plugin should prefer preserving the API author's intent over inventing a
friendlier-but-less-faithful tool shape.

## Request Mapping

When an MCP client calls a tool, the plugin:

1. validates required parameters
2. maps path parameters into the URI path
3. maps query parameters into the query string
4. maps header and cookie parameters into request metadata
5. attaches `body` when present
6. emits a command-plugin `http-request` back to Restish

The URI uses the form `<apiName><path>?...`, which deliberately routes through
Restish's normal API-resolution path instead of hard-coding base URLs inside
the plugin.

That means auth, profile resolution, request middleware, retries, TLS, and
other Restish behavior still apply to MCP tool calls.

Parameter serialization follows a narrow OpenAPI-compatible subset:

- scalar path, query, header, and cookie values are rendered as scalar text
- query arrays use repeated keys for `style: form, explode: true`
- query arrays may also use comma, space, or pipe joining for the matching
  OpenAPI array styles
- header arrays use the OpenAPI `simple` style and are comma-joined
- object parameters and unsupported array styles fail the tool call with a
  clear MCP error instead of sending Go debug strings such as `[a b]`

MCP uses the same internal OpenAPI parameter serializer as generated CLI
commands after it validates the JSON tool argument shape. This keeps percent
encoding, style defaults, and explode handling from drifting between the human
CLI and model-facing tool surfaces.

Host-resolved operations should carry parameter location, type, item type,
style, explode, and allow-reserved metadata so the MCP plugin does not need to
re-parse raw OpenAPI documents when Restish has already resolved the operation.

## MCP Surface Area

The plugin currently implements a focused stdio MCP server:

- `initialize`
- `notifications/initialized`
- `ping`
- `tools/list`
- `tools/call`

HTTP transport is intentionally not part of the v2 MCP plugin surface. The
current design is stdio-first, with `restish mcp serve <api...>` as the service
entry point.

The stdio JSON-RPC reader enforces both payload and header limits. Payloads are
capped at 64 MiB, individual header lines are capped at 8 KiB, and the full
header preamble is capped at 16 KiB. Invalid JSON-RPC frames receive parse or
invalid-request errors (`-32700` or `-32600`) instead of being silently dropped;
requests without a usable ID use JSON-RPC `id: null` in the error response.

## Result Shaping

Tool results are returned as MCP text content. By default the plugin formats
the normalized HTTP body as pretty JSON, with a few deliberate choices:

- HTTP 4xx/5xx results are marked as MCP errors and include the status code
- `201 Created` responses with a `Location` header include a larger envelope
  with status, headers, and body
- large results are truncated to `--max-result-bytes`

This keeps the MCP response concise for model-oriented tool use while still
surfacing a small amount of HTTP context when it matters.

The plugin is intentionally not trying to replicate the full Restish terminal
presentation or raw HTTP envelope in tool results.

## Error Model

The plugin has three main error classes:

### Startup Errors

- API spec could not be loaded
- spec had no usable operations
- command flags are invalid

### Tool-Mapping Errors

- required input missing
- parameter coercion invalid
- selected operation disallowed by plugin policy

### Execution Errors

- delegated HTTP returned failure status
- host/plugin session broke
- result exceeded output constraints

The plugin should keep these categories distinct so operators can tell whether
an issue is with configuration, tool invocation, or the underlying API call.

## Why `x-mcp-ignore` Exists

`x-mcp-ignore` is important because MCP exposure is not the same as CLI
exposure.

Some operations may be:

- valid for direct CLI use by a human operator
- too dangerous, noisy, or semantically odd as AI tools

That is why `x-mcp-ignore` exists separately from `x-cli-ignore`.

## Alternatives Considered

### Build MCP Support Directly Into The Main Restish Binary

Possible, but less valuable as an architectural validation of the plugin model.

### Have The Plugin Parse API Configs And Perform HTTP Itself

Would duplicate core Restish behavior and weaken the point of using a command
plugin.

### Expose Every HTTP Detail In MCP Results

Too noisy for tool consumption.

## Relationship To Other Designs

- Design 007 defines how API operations are generated from specs.
- Design 020 defines the generic command-plugin session this plugin uses.
- Design 029 defines the delegated request pipeline the plugin depends on.
- Design 031 treats `x-mcp-ignore` and stable operation naming as part of the
  compatibility story.
