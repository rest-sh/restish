# Plugin Architecture Overview

## Summary

Restish v2 supports out-of-process executable plugins discovered as
`restish-<name>`. Plugins extend the CLI while reusing host-managed config,
request execution, output planning, and other shared behavior where appropriate.

The plugin system is intentionally split into several protocol families because
the extension use cases have very different lifecycle and trust requirements:

- hook plugins
- command plugins
- TLS signer plugins

## Goals

- let contributors extend Restish without rebuilding the main binary
- keep plugin protocols small and purpose-specific
- preserve host ownership of the core request pipeline
- support multiple languages through a simple CBOR-over-stdio transport
- keep plugin discovery deterministic and operator-controllable

## Non-Goals

- sandboxing arbitrary untrusted plugin code
- one giant generic plugin protocol for every use case
- allowing plugins to replace the host CLI architecture wholesale

## Discovery Model

Plugins are discovered from:

1. executables named `restish-*` in the configured plugin directory

Plugin discovery intentionally does not scan `PATH`; installing a plugin into
the configured directory is the explicit trust decision. If multiple plugins
claim the same manifest name or command name, Restish must not silently pick one
without surfacing the collision.

Plugin installation treats the user's source argument as the trust decision.
Before copying a binary or archive into the configured plugin directory,
`plugin install` shows the source, resolved binary or archive, manifest
identity, and declared capabilities. Automation can opt in with `--yes`, but
the runtime still fails closed: a plugin must declare a hook, loader,
formatter, signer, or command before Restish enables that capability.

Accepted install sources are intentionally explicit:

- a local file path
- an executable already resolvable on `PATH`
- an `http` or `https` URL pointing at a plugin binary, `.zip`, `.tar.gz`, or
  `.tgz`
- a GitHub latest-release shorthand of the form `owner/repo plugin`, which
  resolves one release asset for the current `GOOS`/`GOARCH`

The GitHub latest-release shorthand is part of v2 scope because it materially
improves plugin install usability. It should not be treated as a speculative
future source format unless implementation uncovers a security or packaging
blocker that needs a separate design decision.

Archive extraction must be defensive. The installer reads archives into a
bounded temporary directory, accepts only regular files whose basename matches
the requested plugin name or the `restish-*` plugin convention, flattens archive
paths before writing, enforces download/member/extracted-size limits, rejects
archives with zero or multiple plugin candidates, and only then copies the
selected executable into the plugin directory as `restish-<manifest-name>`.
The copied file is re-queried for its manifest; if that check fails, the
installed file is removed.

This installation flow is a trust prompt, not a software-supply-chain proof.
Plugins run at the user's own risk. Direct URLs and GitHub latest-release
shorthands do not verify publisher identity, pin immutable artifacts, or make
remote code safer than running any other downloaded executable. V2 should not
add extra remote-source restrictions or warnings beyond the normal install
confirmation and `--yes` automation path. The security model already treats
installed plugins as trusted local executables, and the install UX must not
imply otherwise.

The configured plugin directory comes from the same path resolver as the rest
of Restish config. It should not have a separate helper that accidentally
ignores XDG or test path overrides.

## Manifest

Each discovered candidate is queried for a manifest. The manifest is the
plugin's declaration of identity and capability, including at least:

- name
- version
- description
- Restish protocol/API version
- declared hooks
- formatter names
- loader content types

Manifest compatibility is checked before the plugin is allowed to participate in
runtime behavior.

The compatibility policy is intentionally additive:

- `restish_api_version` is the minimum host/plugin API version the plugin
  requires, not the version it happened to be built with.
- future protocol versions must remain backward compatible with older
  manifest meanings.
- unknown optional manifest fields are ignored.
- `required_features` is the fail-closed mechanism for additive behavior that
  a plugin cannot operate without.
- breaking changes require a new major protocol family rather than changing
  the meaning of an existing field.

Plugins that ask for an API version newer than the host supports, declare an
unknown hook, or list an unsupported required feature are rejected during
manifest loading with a plugin-specific diagnostic. Plugins that only include
new optional fields remain loadable.

Manifest discovery may use an on-disk cache, but the cache key must include at
least executable path, modification time, and binary size. Cache writes are
atomic same-directory temp-file renames so a crash during discovery does not
leave a corrupt cache that hides fresh plugin manifests.

Known hook names are `auth`, `request-middleware`, `response-middleware`,
`loader`, `formatter`, `command`, and `tls-signer`. Hook-specific manifest
fields are part of that declaration: formatter plugins must declare
`formatter_names`, loader plugins must declare `loader_content_types`, and
those fields are rejected when the corresponding hook is absent.

Request-middleware header updates use host-owned merge semantics: a string
sets one header value, an array replaces all values, and `null` deletes the
header. Response-middleware `response_headers` replaces the normalized response
header object rather than merging into it; plugins that need to preserve inbound
headers must echo them back explicitly.

Manifest and startup protocol fields are part of the public Go API for plugin
authors. They should have godoc that explains behavior and compatibility
expectations, and exported constants should be used for host-provided startup
flags instead of duplicating string literals across plugins.

Startup flags are an internal argv prefix contract. The Restish host may inject
`--rsh-plugin-manifest`, `--rsh-plugin-commands`, terminal context flags, or
future `--rsh-*` startup fields before the first user argument. Public plugin
helpers must only inspect that contiguous prefix. Once the first non-startup
argument appears, later `--rsh-*` tokens belong to the user's command and must
not trigger manifest mode or spoof host terminal state.

## Transport

The host/plugin transport uses self-delimiting CBOR messages over stdio.

This keeps the transport:

- language-agnostic
- binary-safe
- simple to debug with helper tooling

Plugins should not need custom framing logic beyond CBOR item encoding and
decoding.

## Trust Model

Plugins are trusted local executables, not sandboxed untrusted code. The host
still owns several safety checks:

- version compatibility
- command-name collision handling
- timeout and cleanup policy
- protocol-level scoping of what each plugin type may request

Design 030 defines the broader trust model.

## Host Ownership

Even with plugins, the Restish host remains responsible for:

- config load and validation
- request planning and execution
- auth/TLS/cache/retry semantics
- output planning
- diagnostics and redaction
- subprocess lifecycle cleanup

Plugins are additive seams in the host pipeline, not alternate implementations
of the whole product.

The canonical public module path for plugins is
`github.com/rest-sh/restish/v2`. Documentation and examples should compile
against that module path rather than an old repository path or internal
packages.

## Future Plugin Candidates

Future plugin ideas are useful design probes. They help keep the protocol broad
enough for real extensions without turning the host into a generic automation
runtime. The examples below should stay in mind when evaluating compatibility,
message shapes, config access, streaming behavior, and helper APIs.

Auth and credential plugins:

- AWS SigV4, custom HMAC, Hawk, or other request-signing schemes.
- OAuth provider-specific token exchange, SSO, and device-code variations that
  are too policy-heavy for the built-in auth handlers.
- Vault, 1Password, pass, keychain, or cloud secret-manager token fetchers.
- API-specific auth refreshers that need host-owned prompting and redaction.

Request and response middleware plugins:

- Provider-specific pagination strategies, including page/count APIs and
  continuation tokens that are not exposed as standard links.
- `202 Accepted` polling workflows that follow operation-status URLs until a
  terminal response is available.
- Correlation IDs, idempotency keys, audit headers, and organization-specific
  request metadata.
- Error-envelope normalizers that convert provider-specific problem responses
  into a stable shape before formatting.

Loader plugins:

- Swagger/OpenAPI 2.0 conversion into OpenAPI 3.x.
- Postman, Insomnia, Bruno, or other collection formats converted into OpenAPI.
- GraphQL introspection conversion experiments for API-aware command
  generation.
- Vendor catalog formats that need a small translation layer before Restish can
  use the normal OpenAPI path.

Formatter plugins:

- Markdown tables, GitHub-flavored issue/comment output, and human-readable
  reports for docs or tickets.
- NDJSON, Prometheus exposition, JUnit, TAP, or other automation-oriented
  formats.
- HTML or static report generation for responses that need to be shared outside
  a terminal.
- Domain-specific renderers for logs, metrics, traces, or audit events.

Command plugins:

- API smoke tests, workflow checks, and contract probes driven by registered
  APIs.
- Light rate-limit, retry, or load-test experiments that need pacing,
  concurrency, and summary reporting.
- API diff, changelog, or compatibility-report commands.
- Mock-server generation, SDK snippet generation, or OpenAPI linting workflows.
- Import/export commands for collections or profile setup.

TLS signer plugins:

- macOS Keychain, Windows CNG, TPM, SSH agent, cloud KMS, and other external
  signing backends.
- Enterprise signers that require an external approval or token-session
  lifecycle.

These candidates put pressure on several protocol questions:

- request-signing plugins may need access to request bodies, body hashes,
  canonical header ordering, or the final URL after all host preparation;
- response-follow plugins may need headers and request bodies, not only method
  and URI;
- loader plugins need enough source metadata to produce useful diagnostics and
  choose the right conversion path;
- command plugins need ergonomic helpers for every supported host capability,
  not only delegated HTTP and API spec loading;
- compatibility negotiation needs to distinguish "this plugin was built with a
  newer API" from "this plugin requires a feature this host cannot provide."

## Why Separate Plugin Types

The split is intentional:

- hook plugins are mostly short-lived and bounded
- command plugins are conversational workflow sessions
- TLS signer plugins proxy private-key operations for TLS

Keeping those distinct makes each contract much easier to specify and keeps the
failure modes easier to reason about.

## Lifecycle Expectations

Different plugin categories have different lifecycles, but some rules are
universal:

- all plugin processes are tied to a host-owned context
- the host surfaces plugin stderr when it is helpful for debugging
- plugin stderr should be serialized by line when it shares a destination with
  host diagnostics
- the host should not wait forever on a hung plugin
- successful completion still requires process cleanup
- per-request hook plugins should start in under 100 ms; expensive work should
  move to command plugins or a future long-lived hook design

The category-specific docs define the exact session model.

## Alternatives Considered

### One Generic Bidirectional Protocol

Rejected because it would make small plugins too heavy and lifecycle guarantees
too vague.

### In-Process Dynamic Plugins

Rejected because executable plugins are easier to ship, isolate, and debug.

### Library Extension Only

Too limiting for contributors who do not want to ship a custom binary.

## Relationship To Other Designs

- Design 019 defines hook plugin behavior.
- Design 020 defines command plugin sessions.
- Design 021 defines TLS signer plugins.
- Design 029 defines the shared request pipeline that plugins may delegate to.
- Design 030 defines plugin trust boundaries and redaction obligations.
