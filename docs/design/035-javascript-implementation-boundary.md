# JavaScript Implementation Boundary

Status: accepted boundary; review before the first stable Restish v2 release.

## Problem

The old v1 source tree is no longer part of this repository, and the unreleased
WASM prototype was removed from the working branch to keep the v2 codebase
focused. The remaining JavaScript used by the docs site and browser playground
is user-facing, but it is not part of the Go CLI runtime and must not become a
second source of truth for CLI behavior.

## Boundary

The Go CLI and these design records own command names, flags, output contracts,
OpenAPI behavior, and request execution semantics. Site JavaScript may
demonstrate or render those behaviors, but it should consume documented
examples or generated data where practical rather than reimplementing CLI
rules.

Embedding lifecycle behavior, including whether a `CLI` instance installs
process-level SIGINT/SIGTERM handling, is also owned by the Go API. Site or
playground JavaScript must not duplicate signal-control semantics or present
them as browser-runtime behavior.

## Review Scope

Review the JavaScript implementation and docs-site integration for:

- stale v1 command names or flags
- assumptions about generated command layout
- browser playground request rendering and output behavior
- docs examples that duplicate CLI behavior in JavaScript
- accessibility, progressive enhancement, and failure states

## Non-goals

- Do not reintroduce `cmd/restish-wasm`.
- Do not vendor old v1 source into this repository.
- Do not make JavaScript the source of truth for CLI behavior.

## Outcome

Keep this as a boundary record rather than a general backlog item: the docs site
can have JavaScript, but Restish v2 behavior lives in the CLI, user docs, and
design records. The release-readiness pass should remove stale v1 command names
or duplicated CLI behavior from the site before v2 is stable.
