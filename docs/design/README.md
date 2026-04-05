# Restish v2 Design Records

These documents capture the design intent behind major Restish v2 features and
cross-cutting decisions.

They are primarily for contributors and AI agents working on the codebase.
They are not meant to be polished end-user documentation; the future docs site
can build on these records, but does not need to mirror their structure.

They are intentionally lightweight:

- one file per major feature or design area
- written in Markdown
- optimized for quick reading by humans and predictable parsing by AI agents
- focused on why the feature exists, what shape we chose, and what tradeoffs we
  accepted

The format is not rigid. Most records should stay short and use only the
sections that help explain the design clearly.

The order below is intentional. It starts with the highest-level core ideas,
then moves through request construction and API-aware behavior, then response
handling and operator workflows. Each document should ideally rely only on
concepts introduced earlier in the sequence.

**Foundations**

- [001-cli-architecture.md](/Users/daniel/src/restish2/docs/design/001-cli-architecture.md) - Central `CLI` object, reduced global state, and registry-based composition.
- [002-config-and-profiles.md](/Users/daniel/src/restish2/docs/design/002-config-and-profiles.md) - Single-file config model, API registrations, and profile layering.
- [003-content-types-and-encodings.md](/Users/daniel/src/restish2/docs/design/003-content-types-and-encodings.md) - Registry-driven body encoding/decoding and compression handling.

**Request And API Model**

- [004-authentication.md](/Users/daniel/src/restish2/docs/design/004-authentication.md) - Profile-driven auth handlers, prompting, token caching, and request injection.
- [005-tls-and-cert-handling.md](/Users/daniel/src/restish2/docs/design/005-tls-and-cert-handling.md) - TLS configuration, mTLS options, custom CAs, and certificate inspection.
- [006-spec-discovery-and-loading.md](/Users/daniel/src/restish2/docs/design/006-spec-discovery-and-loading.md) - How Restish finds, parses, and caches API specs.
- [007-api-command-generation.md](/Users/daniel/src/restish2/docs/design/007-api-command-generation.md) - Config-backed API registration and OpenAPI-driven command generation.
- [008-shorthand-input.md](/Users/daniel/src/restish2/docs/design/008-shorthand-input.md) - Building request bodies from CLI arguments and stdin using shorthand syntax.

**Response And Data Flow**

- [009-response-normalization-and-output.md](/Users/daniel/src/restish2/docs/design/009-response-normalization-and-output.md) - The normalized response model and output behavior across TTY and non-TTY use.
- [010-filtering-and-projection.md](/Users/daniel/src/restish2/docs/design/010-filtering-and-projection.md) - Response querying with shorthand and jq, including auto-detection and raw output.
- [011-pagination-and-hypermedia.md](/Users/daniel/src/restish2/docs/design/011-pagination-and-hypermedia.md) - Link extraction, automatic pagination, and collection handling across pages.
- [012-streaming.md](/Users/daniel/src/restish2/docs/design/012-streaming.md) - SSE and NDJSON streaming behavior, per-event filtering, and output rules.
- [013-caching-and-retries.md](/Users/daniel/src/restish2/docs/design/013-caching-and-retries.md) - HTTP response caching, transport layering, and retry behavior.
- [025-image-rendering.md](/Users/daniel/src/restish2/docs/design/025-image-rendering.md) - Terminal image rendering for image/* responses: Kitty, iTerm2, and half-block fallback.

**Workflows And UX**

- [014-edit-workflow.md](/Users/daniel/src/restish2/docs/design/014-edit-workflow.md) - Fetch-edit-update flow, diff review, and patch support.
- [015-links-command.md](/Users/daniel/src/restish2/docs/design/015-links-command.md) - Inspecting normalized hypermedia links directly from responses.
- [016-setup-and-completions.md](/Users/daniel/src/restish2/docs/design/016-setup-and-completions.md) - Shell setup, noglob aliases, and completion behavior.
- [017-cli-behavior-and-diagnostics.md](/Users/daniel/src/restish2/docs/design/017-cli-behavior-and-diagnostics.md) - Exit codes, silent mode, verbose output, and command-line behavior conventions.

**Extensibility**

- [018-plugin-architecture-overview.md](/Users/daniel/src/restish2/docs/design/018-plugin-architecture-overview.md) - Discovery, manifests, plugin categories, and the relationship to the in-process registry model.
- [019-hook-plugins.md](/Users/daniel/src/restish2/docs/design/019-hook-plugins.md) - Short-lived auth, middleware, loader, and formatter plugins.
- [020-command-plugins.md](/Users/daniel/src/restish2/docs/design/020-command-plugins.md) - Long-lived workflow commands that delegate HTTP and formatting back to Restish.
- [021-tls-signer-plugins.md](/Users/daniel/src/restish2/docs/design/021-tls-signer-plugins.md) - External mTLS signing for hardware-backed or otherwise non-exportable client keys.
- [022-restish-pkcs11-plugin.md](/Users/daniel/src/restish2/docs/design/022-restish-pkcs11-plugin.md) - The concrete PKCS#11 TLS-signer plugin, including token selection, PIN sourcing, and crypto11 integration.
- [023-restish-mcp-plugin.md](/Users/daniel/src/restish2/docs/design/023-restish-mcp-plugin.md) - The concrete MCP command plugin that exposes OpenAPI operations as MCP tools over stdio.
- [024-restish-bulk-plugin.md](/Users/daniel/src/restish2/docs/design/024-restish-bulk-plugin.md) - The concrete bulk-management command plugin that revives the v1 checkout workflow out of process.
