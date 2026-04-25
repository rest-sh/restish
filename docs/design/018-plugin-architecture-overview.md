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
- the host should not wait forever on a hung plugin
- successful completion still requires process cleanup

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
