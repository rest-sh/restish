# `restish-mcp` Plugin

## Summary

`restish-mcp` exposes one or more registered Restish APIs as MCP tools over
stdio. It is the concrete command-plugin example that validates the generic
command-plugin protocol against a real integration target.

Its job is to translate cached OpenAPI operations into MCP tool definitions,
then delegate actual HTTP execution back to Restish.

## Problem

The generic command-plugin design explains the transport, but `restish-mcp`
adds a second layer of product decisions that are worth documenting directly:

- which OpenAPI operations become tools
- how tool names are derived and namespaced
- how request arguments map onto HTTP requests
- how much HTTP response detail gets surfaced to MCP clients
- how stdio and plugin messaging interact

Those choices shape whether the plugin feels like a thin protocol adapter or a
full parallel API client.

## Design

The plugin advertises:

- `name: mcp`
- `hooks: ["command"]`

and contributes a single command declaration:

- `mcp`

That command opts into `passthrough_stdio`, which is what allows the plugin to
speak JSON-RPC over stdio to an MCP client while still living inside Restish's
command-plugin transport.

## Lifecycle

The lifecycle has two nested protocols:

1. Restish and the plugin speak the command-plugin CBOR protocol.
2. The plugin and the MCP client speak JSON-RPC over framed stdio.

At startup:

1. Restish sends the command-plugin `init` message.
2. The plugin parses command flags and API names.
3. The plugin asks Restish for each API spec using `api-spec`.
4. The plugin converts the specs into MCP tool definitions.
5. The plugin starts serving MCP stdio traffic.

This design keeps spec loading and HTTP transport inside Restish while letting
the plugin own the MCP-facing protocol.

## Tool Generation

`restish-mcp` loads tools from cached or discoverable OpenAPI specs and only
exposes operations that have an `operationId`.

Operations are skipped when:

- `x-cli-ignore` is true
- `x-mcp-ignore` is true
- `--read-only` is set and the method is not `GET` or `HEAD`
- `--operations` is set and the operationId is not allowlisted

When serving multiple APIs at once, tool names are namespaced as:

- `<apiName>__<operationId>`

That avoids collisions while keeping single-API use ergonomic.

The generated MCP input schema is built from:

- OpenAPI parameters mapped by name
- request-body schema exposed as a `body` property
- the operation summary or description for tool help text

This is intentionally close to the OpenAPI shape rather than a custom MCP-only
abstraction.

## Request Execution

When an MCP client calls a tool, the plugin:

1. validates required parameters
2. maps path, query, header, and cookie params into an HTTP request
3. attaches `body` when present
4. emits a command-plugin `http-request` back to Restish

The URI uses the form `<apiName><path>?...`, which deliberately routes through
Restish's normal API-resolution path instead of hard-coding base URLs inside
the plugin.

That means auth, profile resolution, request middleware, retries, TLS, and
other Restish behavior still apply to MCP tool calls.

## MCP Surface Area

The plugin currently implements a focused stdio MCP server:

- `initialize`
- `notifications/initialized`
- `ping`
- `tools/list`
- `tools/call`

The `--http` transport flag exists but is explicitly not implemented yet. The
current design is stdio-first.

Tool results are returned as MCP text content. By default the plugin formats
the normalized HTTP body as pretty JSON, with a few deliberate choices:

- HTTP 4xx/5xx results are marked as MCP errors and include the status code
- `201 Created` responses with a `Location` header include a larger envelope
  with status, headers, and body
- large results are truncated to `--max-result-bytes`

This keeps the MCP response simple while still surfacing a small amount of HTTP
context when it matters.

## Why It Matters Architecturally

`restish-mcp` is more than "one more plugin." It demonstrates that the
command-plugin system is expressive enough to support:

- plugin-defined top-level commands
- spec discovery delegated back into Restish
- HTTP execution delegated back into Restish
- passthrough stdio for a second protocol layered on top

If this plugin became awkward, that would be a sign the generic command-plugin
design was missing an important capability. In practice it is a strong
integration test for that architecture.

## Alternatives Considered

### Build MCP support directly into the main Restish binary

Possible, but it would make the core CLI own another protocol surface and would
be less useful as a validation of the plugin system.

### Have the plugin parse API configs and perform HTTP itself

That would duplicate core Restish behavior and weaken the architectural point
of using a command plugin in the first place.

### Expose every HTTP detail in MCP results

That would be more faithful to the wire, but noisier for model-oriented tool
consumption. The current design prefers concise tool results with selective
envelope inclusion.

## Notes

The implementation lives in
[`cmd/restish-mcp/main.go`](../../cmd/restish-mcp/main.go)
and
[`cmd/restish-mcp/mcp.go`](../../cmd/restish-mcp/mcp.go),
with coverage in
[`cmd/restish-mcp/mcp_test.go`](../../cmd/restish-mcp/mcp_test.go).

One detail worth preserving is that the plugin depends on `operationId` as the
stable tool identifier. That keeps names predictable and aligns the MCP surface
with the API author's intended operation naming.
