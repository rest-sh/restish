# Spec Discovery And Loading

## Summary

Restish v2 discovers API descriptions through a bounded, security-aware probe
process, parses them through registered loaders, and caches the raw documents on
disk for later command generation.

This is one of the most important architectural seams in Restish because it
connects:

- user-configured API registrations
- network discovery
- plugin-provided loaders
- generated command surfaces

## Goals

- work well when the user provides an explicit spec URL or file
- make conventional OpenAPI publication easy to discover automatically
- keep CLI startup offline-safe by using cached results
- allow more spec formats in the future
- defend against SSRF and hung discovery probes

## Non-Goals

- requiring live network access just to build the command tree
- tying the rest of the CLI permanently to one OpenAPI parsing library
- treating discovery as "best effort" with silent failure

## Three Stages

The design has three explicit stages:

1. discovery
2. loading
3. caching

They should stay conceptually separate even if some helpers combine them in the
implementation.

## Discovery

Discovery answers: "Where should Restish look for a spec?"

The ordered strategy is:

1. local cache for the API registration
2. explicit `spec_files`
3. explicit `spec_url`
4. advertised spec links discovered from the API base URL
5. well-known OpenAPI paths
6. body of the base URL response when the response itself appears to be a spec

The exact probe list may evolve, but explicit operator intent should always win
over heuristics.

An explicit `spec_url` is authoritative once it is configured. Refresh flows
such as `api sync` fetch that URL directly and do not race it against
well-known paths or body/link discovery probes. This matters when an API serves
an older or different spec from a heuristic endpoint: changing `spec_url` must
change the source Restish trusts.

## Discovery Safety Rules

Discovery touches untrusted remote content and can induce more network traffic,
so it must obey design 030.

The design requirements are:

- default timeout for every discovery request
- same-origin discovery by default
- explicit opt-in before following cross-origin discovery links
- reject private-range or loopback follow targets by default unless the original
  configured API is itself in that trust class
- treat DNS lookup errors and lookup timeouts as non-public when validating
  cross-origin discovery targets
- only `http` and `https` are valid remote schemes
- size limits before full-body parse attempts
- all requests derive from the command context

Spec discovery is not a place for indefinite waits or unconstrained redirects.
The current implementation resolves hostnames before fetching but does not yet
dial by the resolved IP literal, so a DNS rebinding race between validation and
transport dial remains a residual risk for cross-origin discovery. Cross-origin
follow is opt-in and unknown/private targets fail closed.

## Probe Execution

Probes may run in parallel for latency, but the selection rules must remain
deterministic and debuggable.

Good parallelism:

- checking several well-known paths concurrently
- trying several same-priority discovery sources concurrently

What must remain deterministic:

- explicit config sources beat heuristics
- error reporting is not order-dependent
- cancellation stops all outstanding probes

If multiple probes succeed, precedence should be based on source class and
loader confidence, not whichever goroutine happened to finish first.

## Loading

Loading answers: "What format is this document, and can Restish convert it into
the canonical internal API model?"

The loader contract should conceptually be:

- accept raw bytes plus content-type and origin metadata
- detect whether the loader understands the document
- return a canonical API description or a canonical OpenAPI document that the
  core can continue processing

The long-term design direction is to decouple command generation from
parser-library-specific types. The rest of the product should consume a stable
operation model rather than whichever concrete parser the loader happened to
use.

### Built-In OpenAPI Loader Contract

The built-in OpenAPI loader accepts OpenAPI 3.0 and 3.1 documents encoded as
JSON or YAML. It should recognize conventional OpenAPI media types, plain
structured media types, and documents whose top-level `openapi` key appears
after other keys.

The loader must receive origin metadata, not just bytes. The metadata includes:

- source URL for network specs;
- local path for file specs;
- request context;
- HTTP transport;
- whether cross-origin external references are allowed.

OpenAPI `$ref` resolution is part of loading because command generation needs a
fully usable operation model. Supported references include local relative files,
full `file://` URIs from local specs, same-origin remote URIs, and cross-origin
remote URIs only when explicitly enabled and permitted by design 030. Remote
specs must not be able to read local filesystem refs. Reference fetches inherit
the discovery timeout, context, size-limit, redirect, and private-host safety
rules.

External references may appear in Path Items, parameters, request schemas,
response schemas, and shared component schemas. Missing or blocked references
should identify the failing reference and source document in diagnostics.

Webhooks-only documents, documents without paths, and empty Path Items are valid
inputs that generate no request commands. They must not panic loader,
generation, MCP tool export, or help registration paths.

When `spec_files` contains multiple documents, Restish deep-merges the parsed
documents and re-serializes the merged result as YAML before loading. This keeps
the merge deterministic but means YAML anchors, aliases, comments, and exact
scalar spelling are not preserved across the merge boundary. Single-file loads
avoid this round trip.

Spec discovery recognizes `Link` relations `service-desc`, `service-doc`, and
`describedby` as advertised API description links.

## Loader Selection

Loaders are registered in priority order. Sources include:

- built-in loaders
- plugin-provided loaders

Selection rules should be explicit:

- content-type hints may narrow candidates
- detection may inspect bytes
- the winning loader should be the highest-confidence/highest-priority loader
  that accepts the document

Plugin loaders should not need to emulate the core parser's internal types.

## Caching

Restish caches raw spec documents, not parsed internal structures.

The cache entry should include at least:

- fetched raw bytes
- content type
- source URL or origin metadata, including whether it came from explicit
  `spec_url`, local `spec_files`, or heuristic discovery
- fetch time
- Restish version or cache schema version

Caching raw documents keeps the cache stable while letting parsing logic evolve
with the binary.

Startup also caches extracted operation metadata alongside the raw bytes as a
performance optimization. That cache is a command-generation artifact, not a
replacement for the raw spec cache. It is keyed by the API base URL, operation
base, effective OpenAPI server-variable values, cache schema version, Restish
version, raw spec hash, and local spec file freshness. Routine startup can build
generated commands from this operation cache without invoking the OpenAPI parser.
Rare flows that need the full document, such as `api inspect` or plugin `api-spec`
requests, parse the raw bytes lazily on demand.

OpenAPI server variables are intentionally part of operation-cache identity.
Changing `server_variables` in API config, or profile-level overrides, can change
generated operation paths even when the raw spec bytes are unchanged.

The operation cache is the normal startup path for large APIs. Command tree
construction should not invoke the OpenAPI parser or fetch external references
when valid operation metadata exists. Cold parsing belongs to explicit sync,
configuration, and cache-refresh paths.

## Cache Validity

A cache entry is valid when:

- it is within TTL or otherwise still acceptable for the command
- it matches the expected cache/schema version
- it is not older than an explicitly configured local source file
- it matches the configured authoritative source identity

A cache entry fetched from heuristic discovery must not satisfy a later
configuration with explicit `spec_url`, even if both describe the same API base.
Likewise, changing `spec_url` invalidates the previous raw-spec and operation
metadata caches for that API.

Local `spec_files` must win over stale cached network content. If the user
edited a local file, Restish should not continue serving a cached older
interpretation for a day.

## Cache Use By Command Generation

Generated command registration at startup should use cached or local spec data
only. It must not trigger live network discovery.

If a cached spec cannot be parsed anymore:

- the API should not silently disappear
- Restish should surface a clear warning or error identifying the API and cause

Cache filenames and paths derived from API or profile names must validate those
names before writing or reading. Names containing path separators, `.` or `..`
segments, or other traversal-shaped input should be rejected or migrated with a
clear warning.

## `api connect` And `api sync`

Commands that explicitly manage API registrations may bypass or invalidate cache
more aggressively than routine startup.

Examples:

- `api connect` should not blindly trust stale cached `x-cli-config`
- `api connect <name> <url>` normalizes schemeless URLs the same way requests
  do: `https://` by default, but `http://` for localhost and loopback targets
- `api connect <name> <url> [setup-expression ...]` may consume
  `prompt.*` expressions as prompt preanswers, then apply ordinary config
  shorthand expressions as final overrides before saving
- `api sync` should refresh from the authoritative source; when `spec_url` is
  configured, that means fetching exactly `spec_url`
- `api set` and `config edit` should invalidate cached specs when fields that
  affect discovery or operation generation change, including `base_url`,
  `spec_url`, `spec_files`, `operation_base`, and OpenAPI server variables

Those commands exist specifically to reconcile config with the current server
state, so using a stale cache without telling the user defeats their purpose.

## Error Reporting

Discovery failures should preserve context:

- what source was tried
- whether it was blocked for safety
- whether the fetch timed out
- whether parsing failed

When several probes fail, Restish should combine or prioritize errors in a way
that is stable and useful rather than depending on goroutine completion order.

## Alternatives Considered

### Require Explicit Spec URLs For All APIs

Simpler, but too inconvenient for a tool whose value includes API awareness.

### Always Fetch The Network On Startup

Rejected because startup must remain offline-safe and predictable.

### Cache Parsed Command Models

Faster in some cases, but much more brittle across parser and binary changes.

## Relationship To Other Designs

- Design 007 consumes the canonical operation model produced here.
- Design 034 defines the OpenAPI-specific loading and operation extraction
  contract in reimplementation detail.
- Design 018 and 019 define plugin loader behavior.
- Design 029 defines how config and discovery interact during command execution.
- Design 030 defines the remote-input safety rules for discovery.
