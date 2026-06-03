---
name: rsh-release-qa
description: Run Restish release readiness QA from main using recent commits, built-in historical risk patterns, docs, and full release gates; report reproducible issues without patching product code
---

# Restish Release QA

Run an evidence-first release readiness pass for Restish. The goal is high
confidence and clear reproducible findings, not implementation work.

## Use This Skill For

- Preparing a Restish release or release candidate from `main`.
- Turning recent commits, docs changes, and historical Restish QA risk patterns
  into a concrete release strategy.
- Running the full pre-release gate, targeted regression probes, built-binary
  smoke checks, and docs validation.
- Reporting blockers, non-blocking risks, unrun checks, and reproduction steps.

## Do Not Use This Skill For

- Patching release-blocking bugs.
- Refactoring, test authoring, or docs edits during QA.
- Making release tags, publishing artifacts, or changing release workflows
  unless the user explicitly asks for those separate actions.

If a bug is found, document it thoroughly and suggest the smallest credible fix.
Do not edit product code. Only write `TODO.md` when the user has explicitly
allowed local follow-up notes for the current run. `TODO.md` is local-only and
must never be staged or committed.

## Core Workflow

1. Confirm the exact release candidate:
   - Check `git status --short`, current branch, and `git rev-parse HEAD`.
   - Fetch `origin` when currentness matters.
   - Prefer testing the fetched `origin/main` or an exact candidate commit. If
     the user's working branch should not move, create a temporary worktree.
2. Determine the diff surface:
   - Find the last release tag or user-provided baseline.
   - Read recent first-parent merges and non-merge commits.
   - Classify touched areas: generated OpenAPI commands, auth/config,
     request pipeline, pagination, plugins, output, docs, packaging, CI.
3. Apply the built-in risk map:
   - Read `references/release-risk-map.md` for the distilled release matrix.
   - Read `references/findings-mining.md` for historical finding classes and
     safe repro patterns.
   - Match recent commits to the risk areas and choose targeted probes.
4. Build a run-specific strategy:
   - Always include the full release gate unless the user explicitly narrows it.
   - Add targeted probes for recent commits and relevant findings.
   - Include user-facing docs checks for docs or command-surface changes.
5. Run the checks:
   - Create a fresh temporary run directory and use isolated config/cache paths
     for built-binary smokes.
   - Prefer harmless local servers, `--rsh-print`, `--rsh-server` overrides,
     localhost failure targets, and read-only public specs.
   - Never use real user credentials or destructive provider calls.
6. Report release readiness:
   - Lead with blockers or "no release blockers found".
   - Include exact candidate commit, commands run, failures, environment issues,
     targeted probes, and residual risk.
   - For each issue, include severity, expected behavior, actual behavior,
     reproduction commands, impact, and a suggested fix direction.

## Full Gate

Use the release gate reference for the canonical command set:
`references/release-gates.md`.

Default full gate:

- `go test ./...`
- `go test -tags=integration ./...`
- Targeted `go test -race` over risk-bearing packages.
- Build `restish` and first-party plugin binaries.
- `go run ./cmd/restish-docgen --check`
- `npm --prefix site ci` with a disposable npm cache when needed.
- `npm --prefix site run build` for the docs site.
- Built-binary smoke checks for help, version, redaction, and at least one
  generated OpenAPI command surface when the release touches CLI/OpenAPI/auth.

Use per-run `GOCACHE` and `GOPATH` paths under the temporary run directory when
the default Go caches are not writable in the sandbox. Report any resulting
network, toolchain-download, or module-download limitation instead of changing
dependencies during release QA.

## Findings Mining

Use `references/findings-mining.md` for distilled historical issue classes,
triage rules, and repro selection.

High or security findings with open or uncertain status are release blockers
until verified fixed or explicitly deferred by the user. Medium findings are
blockers when they affect common workflows, release packaging, data integrity,
credential leakage, or newly changed code.

## Targeted Probe Rules

- Generated OpenAPI changes: connect or load a small public/local spec, inspect
  help, run `doctor api`, and test request construction without provider writes.
- Auth/config changes: check noninteractive setup, missing env diagnostics,
  redaction, profile precedence, and trusted project config boundaries.
- Request pipeline changes: check content negotiation, redirects, retries,
  TLS flags, server overrides, and network-error diagnostics.
- Pagination changes: check page parameters, Link headers, metadata filters,
  max page/item limits, and strict `items_path` behavior.
- Plugin changes: build first-party plugins and run relevant integration/race
  tests; check subprocess lifecycle risks.
- Docs/command-surface changes: run docgen drift and Hugo build; smoke the
  affected help output from a built binary.
- Security/redaction changes: test verbose and non-verbose errors with URL
  userinfo, credential-looking query params, headers, JSON bodies, forms, and
  generated credentials.

## Reporting Template

Use this shape unless the user asks for something else:

```text
Release candidate: <commit> on <branch/ref>
Baseline: <tag or commit>

Readiness: ready / not ready / ready with noted risks

Findings:
- P1/P2/... <title>
  Impact:
  Repro:
  Expected:
  Actual:
  Suggested fix direction:

Checks run:
- <command> - passed/failed/skipped

Targeted probes:
- <scenario> - passed/failed

Residual risk:
- <unrun check, environmental limitation, or deferred finding>
```

If every required check passes and no blocking issue is found, say so clearly.
Do not bury skipped checks or environmental limitations.
