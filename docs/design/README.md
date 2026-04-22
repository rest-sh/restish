# Restish v2 Design Records

These documents capture the design intent behind major Restish v2 features and
cross-cutting decisions.

They are primarily for contributors and AI agents working on the codebase.
They are not meant to be polished end-user documentation; the future docs site
can build on these records, but does not need to mirror their structure.

They should now be treated as implementation-grade design records rather than
light sketches. A contributor should be able to read this corpus and recover:

- product goals and non-goals
- persistent data models and compatibility rules
- request/response execution order
- extension points and lifecycle contracts
- security boundaries and failure handling
- expected user-facing behavior in both TTY and non-TTY use

The format is still not rigid, but "short because the code explains the rest"
is no longer the bar. If behavior matters to correctness, compatibility,
security, or user expectations, it should be captured here explicitly.

The order below is intentional. It starts with the highest-level core ideas,
then moves through request construction and API-aware behavior, then response
handling and operator workflows. Each document should ideally rely only on
concepts introduced earlier in the sequence.

## How To Read This Corpus

For an implementation or reimplementation effort, the recommended reading order
is:

1. read the foundations to understand the runtime shape, config model, body
   model, and security stance
2. read the request and API model to understand how commands are discovered,
   planned, and executed
3. read the response and data-flow records to understand normalization,
   filtering, streaming, pagination, retries, and rendering
4. read workflows and UX to recover interactive behavior, operator contracts,
   setup, and exit semantics
5. read extensibility last so plugin behavior is interpreted in the context of
   the host runtime rather than as a parallel architecture

When two records appear to overlap, the more specialized record should define
the subsystem-specific contract while the broader record explains how that
subsystem participates in the end-to-end pipeline.

## Reimplementation Checklist

A design-driven reimplementation should be able to recover at least the
following from this corpus:

- startup and runtime lifecycle
- persistent configuration files, profile layering, and migration boundaries
- command parsing, resolution, and generated API command behavior
- request-body construction, serialization, transport execution, auth, and TLS
- response decoding, normalization, filtering, formatting, and output framing
- streaming, pagination, retries, cache behavior, and cancellation semantics
- plugin discovery, lifecycle, trust boundaries, and host/plugin responsibility
- operator-facing diagnostics, prompts, shell setup, and exit behavior

If an implementation detail is important to interoperability, security,
compatibility, or user expectations, it should live in one of these records
rather than remaining implicit in code.

**Foundations**

- [000-restish-v1-baseline.md](./000-restish-v1-baseline.md) - Feature inventory of Restish v1, captured as the baseline that informed v2 design.
- [001-cli-architecture.md](./001-cli-architecture.md) - Central `CLI` runtime, subsystem boundaries, lifecycle phases, and invariants for embedding, testing, and teardown.
- [002-config-and-profiles.md](./002-config-and-profiles.md) - Persistent configuration model, path resolution, profile layering, atomic writes, and migration expectations.
- [003-content-types-and-encodings.md](./003-content-types-and-encodings.md) - Registry-driven body encoding/decoding and compression handling.
- [030-security-model-and-trust-boundaries.md](./030-security-model-and-trust-boundaries.md) - Threat model, trust boundaries, sensitive-data handling, and the safety rules that apply across discovery, plugins, auth, and output.

**Request And API Model**

- [004-authentication.md](./004-authentication.md) - Profile-driven auth resolution, OAuth flow design, token storage, refresh semantics, prompting, and auth-plugin integration.
- [005-tls-and-cert-handling.md](./005-tls-and-cert-handling.md) - TLS configuration, mTLS options, custom CAs, and certificate inspection.
- [006-spec-discovery-and-loading.md](./006-spec-discovery-and-loading.md) - Secure spec discovery, loader contracts, caching, revalidation, and failure reporting.
- [007-api-command-generation.md](./007-api-command-generation.md) - Config-backed API registration, OpenAPI-to-command mapping, naming, parameter handling, and compatibility aliases.
- [008-shorthand-input.md](./008-shorthand-input.md) - Building request bodies from CLI arguments and stdin using shorthand syntax.
- [029-request-execution-pipeline.md](./029-request-execution-pipeline.md) - End-to-end request planning, execution order, cancellation, transport layering, normalization, filtering, and rendering.

**Response And Data Flow**

- [009-response-normalization-and-output.md](./009-response-normalization-and-output.md) - The normalized response model and output behavior across TTY and non-TTY use.
- [010-filtering-and-projection.md](./010-filtering-and-projection.md) - Response querying with shorthand and jq, including auto-detection and raw output.
- [011-pagination-and-hypermedia.md](./011-pagination-and-hypermedia.md) - Link extraction, automatic pagination, and collection handling across pages.
- [012-streaming.md](./012-streaming.md) - SSE and NDJSON streaming behavior, per-event filtering, and output rules.
- [013-caching-and-retries.md](./013-caching-and-retries.md) - HTTP response caching, transport layering, and retry behavior.
- [025-image-rendering.md](./025-image-rendering.md) - Terminal image rendering for image/* responses: Kitty, iTerm2, and half-block fallback.

**Workflows And UX**

- [014-edit-workflow.md](./014-edit-workflow.md) - Fetch-edit-update flow, diff review, and patch support.
- [015-links-command.md](./015-links-command.md) - Inspecting normalized hypermedia links directly from responses.
- [016-setup-and-completions.md](./016-setup-and-completions.md) - Shell setup, noglob aliases, and completion behavior.
- [017-cli-behavior-and-diagnostics.md](./017-cli-behavior-and-diagnostics.md) - Command resolution, global flag policy, diagnostics, exit codes, prompts, and operator-facing behavior conventions.
- [031-compatibility-and-migration.md](./031-compatibility-and-migration.md) - v1-to-v2 compatibility goals, intentional breaks, migration path, and release-readiness checklist for user-visible behavior.

**Extensibility**

- [../plugin-quickstart.md](../plugin-quickstart.md) - Fastest path to a working plugin, with small formatter and command-plugin examples.
- [018-plugin-architecture-overview.md](./018-plugin-architecture-overview.md) - Discovery, manifests, plugin trust model, lifecycle ownership, and the relationship to the in-process registry model.
- [019-hook-plugins.md](./019-hook-plugins.md) - Short-lived auth, middleware, loader, and formatter plugins, including timeout, error, and output contracts.
- [020-command-plugins.md](./020-command-plugins.md) - Long-lived workflow commands that delegate HTTP and formatting back to Restish, with message lifecycle and stdio rules.
- [021-tls-signer-plugins.md](./021-tls-signer-plugins.md) - External mTLS signing for hardware-backed or otherwise non-exportable client keys, including signer lifecycle and teardown.
- [022-restish-pkcs11-plugin.md](./022-restish-pkcs11-plugin.md) - The concrete PKCS#11 TLS-signer plugin, including token selection, PIN sourcing, and crypto11 integration.
- [023-restish-mcp-plugin.md](./023-restish-mcp-plugin.md) - The concrete MCP command plugin that exposes OpenAPI operations as MCP tools over stdio.
- [024-restish-bulk-plugin.md](./024-restish-bulk-plugin.md) - The concrete bulk-management command plugin that revives the v1 checkout workflow out of process.
- [026-restish-csv-plugin.md](./026-restish-csv-plugin.md) - The concrete formatter-hook plugin that turns array-shaped responses into CSV.
- [027-comment-preserving-config-edits.md](./027-comment-preserving-config-edits.md) - Comment-preserving config editing, structural patch guarantees, line-ending preservation, and concurrency-safe writes.
- [028-document-and-record-output.md](./028-document-and-record-output.md) - Output framing contracts for document vs record formats across pagination, streaming, filtering, and redirects.
