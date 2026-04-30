# V2 Product Surface Decisions

Status: accepted for the first Restish v2 release.

## Problem

Restish v2 is unreleased, so this is the moment to remove confusing command
surfaces instead of carrying aliases and compatibility shims into the first
stable release. The product should make the first useful request work without
config, then make repeated API work feel like a native shell command generated
from OpenAPI.

## Goals

- Keep generic URL requests useful with no config.
- Keep help, version, completion, and doctor usable when config is broken.
- Make `api connect <name> <url>` the single API setup path.
- Keep non-request command help quiet by default while preserving full request
  flag discoverability through `--help-all`.
- Make plugin and MCP trust explicit.
- Preserve a public Go API for custom branded CLIs and in-process extensions.

## User-Facing Decisions

`restish api connect <name> <url>` owns API setup. It normalizes and saves the
base URL, attempts spec discovery by default, applies OpenAPI-derived
configuration when a spec is found, and still registers the API when discovery
does not find a spec. `--no-discover` performs minimal local registration with
no spec probes. `--spec <url-or-file>` uses an explicit OpenAPI source.
`--replace` allows a rerun to replace generated/default material that would
otherwise be preserved.

No old setup aliases are kept. v2 is not released, and multiple setup verbs
would make scripts, docs, help, and support answers less clear.

Local registration removal is `api remove <name>`. The top-level `delete`
command remains the generic HTTP DELETE verb.

Bootstrap-safe commands are allowed to run without full config, plugin, or
generated-command startup. This includes help, version, completion, setup help,
and doctor. Normal requests, generated commands, plugins, and config-mutating
commands keep strict config validation.

Config recovery uses a diagnostic parse path that can inspect unknown fields
and report dotted paths, line/column positions, closest field suggestions, and
known migration hints without accepting those fields for execution. Strict
parsing remains the only path for normal requests and config mutation.
`restish doctor migrate-v1` is the explicit operator command for running
default-location v1 migration when the normal first-run path is eligible.

`RSH_CONFIG_DIR` defines a clean config root. It does not scan or mutate
platform legacy locations. Automatic v1 migration is limited to the default
platform config path when no v2 config exists.

Default help shows command-relevant flags. Request commands show the daily HTTP
and output flags; generated operation help focuses on operation arguments,
examples, schemas, and `--rsh-generate-body`; non-request commands show core
globals only. `--help-all` expands inherited request globals. A separate
`restish flags` or `restish help flags` command is intentionally deferred for
the first v2 release; the command surface already has `--help`, `--help-all`,
and user docs for global/request flags.

Generated API commands use a flat layout by default. Users and API authors can
opt into first-tag grouping with `command_layout: "tags"` when a spec has a
stable, useful tag taxonomy. There is no `auto` layout mode: guessing from tags
would make command names change when a spec's metadata changes, which is a poor
default for scripts and docs.

Generated operation auth override is named `--rsh-auth`. The OpenAPI term
"security" remains in design and spec handling, but users choose auth.

MCP exposes read-like operations by default. `POST`, `PUT`, `PATCH`, and
`DELETE` are hidden unless `--allow-write-tools` is set. Explicit hide metadata
such as `x-mcp-ignore` remains authoritative.

The MCP plugin should use the public command-plugin client helpers for host
HTTP delegation, API spec fetches, timeouts, stdout/stderr messages, and
passthrough stdio. MCP-specific code owns MCP JSON-RPC and tool mapping only;
it should not carry a second pending-request protocol implementation.

Small internal packages should be collapsed only when ownership is obvious.
The raw filter helper and plugin manifest cache were folded into their natural
owners. `internal/input` and `internal/cache` remain separate packages for now:
input parsing is shared by generic and generated request bodies, and cache
behavior is still a distinct request-execution concern. Further movement should
wait for a broader request executor reshape rather than churn files for size
alone.

Plugin installation is trust-explicit: install shows the source, resolved
binary/archive, manifest identity, and declared capabilities before copying the
binary. Automation uses `--yes`. Runtime behavior fails closed: a plugin must
declare a hook before Restish enables that capability.

Bulk push treats a resource that changed locally and remotely as a conflict.
That includes the remote-delete/local-edit case: status should show both sides,
and push must refuse instead of recreating the deleted remote resource unless a
future design introduces an explicit conflict-resolution workflow.

## Embedding

Restish v2 keeps an embeddable Go API for organizations shipping custom CLIs.
Embedders can construct a CLI, set version/branding through the command layer,
register custom content types, encodings, auth handlers, link parsers, loaders,
formatters, in-memory default config, profiles, and bundled API metadata. User
config wins over bundled defaults with the same API or auth-profile name, so
organizations can ship useful defaults without trapping users. Out-of-process
plugins remain the normal extension path for the stock binary.

## Compatibility

These are breaking changes by design. They reduce the command surface before
v2 becomes stable. Migration docs should teach the new commands directly rather
than preserving transitional aliases.

## Testing And Documentation

Tests should cover bootstrap with invalid config, `api connect` discovery and
no-discovery flows, removed setup command names, quiet help for non-request
commands, generated `--help-all`, MCP write gating, plugin install trust, and
plugin undeclared capability rejection.

User docs should use `api connect`, `api remove`, `--rsh-auth`, `-o table`, and
`api auth inspect` consistently.
