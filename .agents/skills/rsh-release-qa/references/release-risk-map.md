# Restish Release Risk Map

This reference distills the recurring release-QA matrix from prior Restish
real-world API sweeps and regression campaigns. Use it to choose targeted
checks after reading recent commits.

## Highest-Value Release Surfaces

- Generated OpenAPI command discovery and execution.
- Auth planning, credential readiness, redaction, and profile precedence.
- Spec cache, operation cache, `api connect`, `api sync`, and local spec
  freshness.
- Request construction: content negotiation, media-type encoders, query/path
  serialization, redirects, retries, TLS, and server overrides.
- Pagination and hypermedia: page params, Link headers, item extraction,
  limits, metadata filters, and streaming output.
- Output formatting: terminal color, JSON/table stability, scalar utility
  output, plugin formatter behavior, and fixture/reference drift.
- Plugin lifecycle: command plugins, formatter plugins, hook plugins,
  `restish-mcp`, subprocess cleanup, and protocol compatibility.
- User-facing docs and generated command reference.
- Release packaging: GoReleaser config, first-party plugin artifacts, docs site,
  Homebrew/OCI metadata, and version injection.

## Standard Matrix

Run these classes when the changed code or release risk justifies them.

- OpenAPI auth fixtures for env-backed query/header credentials, missing env
  diagnostics, `security: []`, optional anonymous `{}` alternatives, OR/AND
  auth combinations, explicit `--rsh-auth`, and verbose redaction.
- Generated command edge cases for parameter serialization, content negotiation,
  request/response schema help, generated examples, provider drift, and generic
  request controls.
- OpenAPI 3.1 schema rendering for nullable types, `const`, `enum`, `oneOf`,
  `anyOf`, `allOf`, conditionals, `contains`, tuple/prefix items,
  `dependentRequired`, and unsupported annotations.
- Local and public spec loading for malformed JSON/YAML, non-OpenAPI documents,
  external refs, missing refs, webhook-only specs, large modular specs, server
  variables, and operation/path-level servers.
- Request body media types: JSON, form URL encoded, multipart, XML, NDJSON,
  `application/stream+json`, `application/octet-stream`, wildcard raw types,
  protocol-specific raw types, stdin, `@file`, missing files, literal `@`, and
  `--rsh-content-type` overrides.
- Query handling: `deepObject`, `form` object styles, repeated arrays,
  JSON-content query params, free-form filter params, env/flag precedence, and
  explicit `--rsh-query` controls.
- Path handling: simple/label/matrix styles, arrays, slashes, commas, percent
  signs, spaces, question marks, already-escaped input, and pathful
  `--rsh-server` overrides.
- Redaction: URL userinfo, credential query params, headers, cookies, JSON
  bodies, form bodies, verbose request/response traces, network errors,
  redirects, plugin stderr, and ambiguous ordinary parameters.
- Pagination: page-param pagination, strict `items_path`, relative Link next
  URLs, quoted Link parameters with commas, malformed Link targets,
  unsupported schemes, max-page/max-item cutoffs, and standalone `links`.
- Built-ins and utilities: `doctor`, `doctor api`, `plugin list`, `api list`,
  `api inspect`, `config show`, `config path`, `cache info`, `version`,
  output/filter flag support, and invalid global flag validation ordering.
- Config and migration: trusted project config, shared project config without
  secrets, concurrent config mutation, v1-to-v2 migration, malformed legacy
  fields, JSONC editing, config directory modes, and cache cleanup.
- Docs: generated docs drift, command help smoke checks, docs site build,
  social images, README install paths, release packaging docs, and examples
  that are shell-safe.

## Useful Public Spec Families

Use public specs only for safe connect/help/request-construction checks. Avoid
provider writes. Prefer local fixtures when a small repro is enough.

- Petstore / Redocly / SFTPGo: baseline generated commands and anonymous auth.
- OpenAPI Generator Echo Server: query serialization, multipart, XML, NDJSON,
  octet-stream, and mixed parameter styles.
- OpenProject / ownCloud / Pirate Weather: operation servers and delimiter-heavy
  path parameters.
- Mailcow: confirm-password fields and generated example sensitivity.
- Tailscale: free-form/dynamic query filters.
- VictoriaLogs / Ollama: NDJSON and streaming JSON.
- UKHO / TUS / SFTPGo / ComfyUI / Walrus: binary and raw upload bodies.
- Gematik Push Gateway: OpenAPI `mutualTLS`.
- Trigger.dev / Backstage / PowerDNS / Hookdeck: object query and pagination
  shapes.
- Adobe Substance 3D / Speakeasy / Livepeer / MarkItDown: multipart,
  file-upload, and complex generated examples.

## Safe Probe Principles

- Use isolated `--rsh-config` and `RSH_CACHE_DIR`.
- Use fake credentials only.
- Use `--rsh-print`, `--rsh-generate-body`, `--help-all`,
  `--rsh-server https://httpbin.org/anything`, or localhost failure targets
  instead of live provider mutations.
- Verify request shape through headers, verbose output, or echo bodies.
- Treat provider downtime or spec drift as residual risk, not an immediate
  Restish bug, unless local controls reproduce it.
