# JavaScript Implementation Follow-up

Status: planned follow-up before the first stable Restish v2 release.

## Problem

The old v1 source tree and the unreleased WASM prototype were removed from the
working branch to keep the v2 codebase focused. The remaining JavaScript used
by the docs site and browser playground still deserves a separate review,
because it is user-facing but not part of the Go CLI runtime.

## Scope

Review the JavaScript implementation and docs-site integration for:

- stale v1 command names or flags
- assumptions about generated command layout
- browser playground request rendering and output behavior
- docs examples that duplicate CLI behavior in JavaScript
- accessibility, progressive enhancement, and failure states

## Non-goals

- Do not reintroduce `cmd/restish-wasm`.
- Do not restore the removed `restish-src/` tree.
- Do not make JavaScript the source of truth for CLI behavior.

## Outcome

Track this as a dedicated follow-up so cleanup of the Go CLI, plugin protocol,
and docs can proceed without pretending the site JavaScript has had the same
depth of review.
