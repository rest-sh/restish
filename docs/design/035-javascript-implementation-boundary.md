# JavaScript Implementation Boundary

Status: accepted boundary; review before the first stable Restish v2 release.

## Problem

The old v1 source tree is no longer part of this repository, and the unreleased
WASM prototype was removed from the working branch to keep the v2 codebase
focused. The remaining JavaScript used by the docs site and browser playground
is user-facing, but it is not part of the Go CLI runtime and must not become a
second source of truth for CLI behavior.

The docs site's first-click onboarding path is the Tour of Restish. That page
uses browser-runnable examples to show the major workflows before or after a
local install: direct requests, normalized filtering, output formats, request
bodies, edit workflow, pagination, streaming, generated API commands, auth,
profiles, plugins, and scripting. The tour is a product router and demo surface,
not a replacement for the CLI, guides, recipes, or reference pages.

## Boundary

The Go CLI and these design records own command names, flags, output contracts,
OpenAPI behavior, and request execution semantics. Site JavaScript may
demonstrate or render those behaviors, but it should consume documented
examples or generated data where practical rather than reimplementing CLI
rules.

The browser playground may provide narrow fixtures for tour examples that a web
page cannot safely perform, such as local config changes, plugin installation,
editor integration, or long-running streams. Those fixtures must be labeled as
preview behavior in user-facing copy and should stay small enough to explain
the workflow without becoming an alternate runtime.

Generated API commands in the browser playground are a curated map for the
public `api.rest.sh` docs API. The map may translate common operation names,
path parameters, auth defaults, and query flags into live URLs so tour examples
remain editable, but it must not become a generic OpenAPI command generator.

Embedding lifecycle behavior, including whether a `CLI` instance installs
process-level SIGINT/SIGTERM handling, is also owned by the Go API. Site or
playground JavaScript must not duplicate signal-control semantics or present
them as browser-runtime behavior.

## Review Scope

Review the JavaScript implementation and docs-site integration for:

- stale v1 command names or flags
- assumptions about generated command layout
- browser playground request rendering and output behavior
- Tour of Restish examples, especially local-only workflows represented by
  browser fixtures
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
