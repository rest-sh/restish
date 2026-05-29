---
name: rsh-test
description: Write or improve Restish tests with concise, high-density Go coverage, especially CLI behavior tests using OpenAPI specs, mock HTTP servers, real CLI commands, and expected output
---

# Restish Testing

Write tests that prove user-visible behavior and preserve important contracts with as little test code as practical. Prefer scenarios that would catch real regressions over broad coverage for its own sake.

## Use This Skill For

- Adding or improving Go tests in Restish
- Choosing between unit, package-level, CLI behavior, integration, golden, or fuzz tests
- Covering regressions, CLI flows, OpenAPI-generated commands, auth, config, plugins, output formatting, pagination, streaming, caching, filtering, and request construction
- Reviewing whether a proposed test is worth its maintenance cost

## Default Test Strategy

1. Start with `rg` for nearby tests and helpers. Reuse local test vocabulary before inventing new fixtures.
2. Prefer a behavioral CLI test when the change affects users: build a small OpenAPI spec or config, serve responses with `httptest.Server` or a fake transport, run `CLI.Run`, then assert stdout, stderr, request shape, exit/error behavior, and persisted files.
3. Use package unit tests for pure logic, parsers, serializers, and edge cases where a CLI test would be noisy.
4. Use table-driven subtests only when cases share the same setup and assertion shape. Name cases by behavior or bug, not by implementation detail.
5. Use golden files only for stable, reviewable output that is too large or visually structured for inline expectations. Keep fixtures in `testdata/`, and update only when output changes intentionally.
6. Use integration tests or `-tags=integration` when the real binary, plugin subprocesses, shell behavior, or external process lifecycle is the thing being tested.
7. Use fuzz tests for parsers, shorthand/input decoding, OpenAPI schema handling, filters, and other input-heavy code. Seed fuzzers with realistic examples and past crashes; keep deterministic regression seeds runnable under normal `go test`.

## Choose The Test Shape

- User-visible CLI behavior, generated commands, config precedence, output, auth, pagination, streaming, or caching: write a CLI behavior test.
- Request construction only: use fake transport plus `requestRecorder`.
- Real HTTP semantics, redirects, compression, TLS, streaming, or server state: use `httptest.Server`.
- Pure parser/formatter/schema/helper logic: write a package unit test, often table-driven.
- Stable multi-line help, formatter, or tabular output: consider a golden file.
- Plugin protocol, subprocess lifecycle, shell setup, or binary packaging: use integration tests.
- Input-heavy code with many edge cases: add or extend fuzz tests with realistic seeds.

When writing a test from scratch, load `references/patterns.md` for copyable Restish-specific patterns.

## Restish Patterns To Reuse

- In `internal/cli`, prefer `newTestApp`, `newTestCLI`, `testArgs`, `requestRecorder`, `requireContains`, `writeTestFile`, and existing generated-command helpers when they fit.
- Instantiate `CLI` directly instead of shelling out unless subprocess behavior is the point of the test.
- Use `httptest.Server` for realistic HTTP behavior and real URLs; use fake transports for focused request-construction assertions.
- Keep config, cache, home, plugin, and spec files under `t.TempDir()`.
- Use `t.Setenv`, `t.Chdir`, and `t.Cleanup` for process-global state. Do not mark tests parallel when they mutate environment, globals, working directory, shared buffers, plugin paths, or caches.
- Register helper failures with `t.Helper()`.
- Assert exact output when it is stable and meaningful. For JSON, prefer decoding and checking semantic fields when ordering or formatting is incidental.

## Plain Go Over Gherkin

Prefer plain Go tests with a small Restish-specific helper vocabulary over Gherkin/Cucumber-style scenario files.

- Restish CLI tests often need precise Go-native control over `httptest.Server`, fake transports, temp config/cache/plugin paths, `CLI` hooks, stdout/stderr buffers, request bodies, auth state, and persisted files. Go helpers keep those details close to the assertion and make failures easier to debug.
- Gherkin usually moves repeated setup into step definitions without removing the real complexity. Use it only if there is a concrete product need for executable acceptance specs readable and maintained by non-Go contributors.
- When tests start to read like repeated scenarios, improve the Go vocabulary instead: add focused helpers for tiny OpenAPI operation specs, mock API servers that serve both specs and endpoint routes, config/profile/auth constructors, CLI run helpers, JSON stdout assertions, and request assertions for method/path/query/header/body.
- Keep helper APIs scenario-like but explicit. A good test should read as setup, run command, assert observable output/request/state, without hiding important behavior behind broad "Given/When/Then" magic.

## High-Density Test Rules

- One test should usually cover one behavior from input through observable result.
- Combine cases when they differ only by inputs and expected outputs. Split cases when setup or failure meaning differs.
- For review-driven edge cases, first preserve the behavioral boundary, then reduce scaffolding. Table-drive repeated redirect, redaction, cache, migration, or auth-origin cases only when the setup and assertion shape truly match.
- Use focused helpers for repeated config/cache/server setup, but keep the request, persisted state, error, or output assertion visible in the test.
- When covering credential handling, assert both where credentials are applied and where they are not applied, plus what gets persisted and what appears in diagnostics.
- Avoid asserting private call sequences unless ordering is the contract.
- Avoid mocks for HTTP, files, and CLI I/O when standard library fakes or temp resources are clearer.
- Keep fixtures tiny but believable: real OpenAPI fragments, real response headers, real shorthand, real config snippets.
- Cover at least one failure path when the changed code adds validation, fallback, retries, auth, subprocess handling, or parsing.
- Do not add sleeps unless testing time itself. Prefer controllable hooks, zero retry delays, short contexts, or explicit channels.
- Do not chase coverage lines that cannot fail in a meaningful way.
- Inline fixtures under roughly 40 lines. Use `testdata/` for larger stable fixtures. Create shared helpers only after repetition is real.

## Avoid These Tests

- Tests that only prove a helper was called, unless the call is the contract.
- Giant OpenAPI specs when a tiny operation fragment exercises the behavior.
- Full stderr/help assertions when one phrase or structured field is enough.
- Table tests where each row needs unrelated setup or different assertions.
- Sleeps, real network calls, or user home/config writes.
- Mock frameworks for behavior the standard library can model clearly.

## Assertion Style

- Setup failures: `t.Fatalf`.
- Independent mismatches after setup: `t.Errorf` is fine, but keep failure output useful.
- Raw text or line output: exact string when stable; substring checks when surrounding formatting is incidental.
- JSON output: decode and assert semantic fields unless formatting is the feature.
- Help, table, and formatter output: use exact strings or golden files when layout is the contract.
- Errors: assert type/sentinel/exit behavior when available; assert full strings only for user-facing message contracts.
- Diffs: label direction as `-want +got` when using a diff helper already present in the package.

## Review Checklist

- Would the test fail before the fix or feature?
- Does it exercise public behavior before private helpers?
- Is the fixture minimal but realistic?
- Does it cover the most important failure path?
- Does it avoid leaking environment, globals, working directory, buffers, subprocesses, caches, or files?
- Is the failure message enough to debug without rerunning under a debugger?

## Verification

Run the narrowest meaningful package first:

```bash
go test ./internal/cli/...
go test ./internal/request/...
go test ./internal/output/...
```

Then run `go test ./...` for shared behavior. Run `go test -tags=integration ./...` before commits that touch CLI/plugin behavior or real subprocess boundaries. Use `go test -race ./...` when changing concurrency, streaming, subprocess lifecycle, shared buffers, or caches.
