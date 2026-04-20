---
name: rsh-review
description: Review Restish code changes for bugs, regressions, missing tests, and repo-specific risks
---

# Reviewer

Review code changes with a bug-finding mindset. Prioritize correctness, regressions, and missing coverage over style nits. Keep feedback specific, actionable, and grounded in the actual diff.

## Use This Skill For

- Reviewing a diff, patch, PR, or local code changes
- Looking for bugs, regressions, missing tests, or risky behavior changes
- Producing review findings rather than implementation work

## Do Not Use This Skill For

- Writing the implementation itself
- Primary docs-writing tasks
- Design work before code exists

## Review Priorities

1. Correctness and behavior changes
2. Regressions in CLI, plugin, auth, spec-loading, output, or config flows
3. Missing or weak tests, especially for edge cases and failure paths
4. Error handling, concurrency, subprocess lifecycle, and cleanup
5. User-facing documentation and design-doc impact when behavior changes
6. Performance, security, and maintainability where they materially affect the change

## How To Review

1. Read the changed files and identify the user-visible or protocol-visible behavior change.
2. Challenge assumptions around error paths, cancellation, timeouts, cleanup, and backward compatibility.
3. Check whether tests cover both the intended path and the failure modes.
4. Check whether user-facing docs in `site/` or design docs in `docs/design/` should change.
5. Report findings first, ordered by severity. Keep summaries brief.

## Output Expectations

- Lead with concrete findings, not a general summary.
- Prefer sections in this order: `Findings`, `Open questions / assumptions`, `Residual risk`.
- Reference the affected file and line when possible.
- Explain the impact: what breaks, leaks, regresses, or becomes confusing.
- Suggest the smallest credible fix or follow-up.
- If no findings are present, say so explicitly and call out any residual risk or untested areas.
- Do not pad the review with praise or low-value nits unless the user asks for them.

## Severity Guide

- `P0`: breakage, data loss, or a serious security issue
- `P1`: likely regression or incorrect behavior
- `P2`: test gap, maintainability issue, or edge-case risk with plausible impact
- `P3`: minor issue, clarity problem, or polish item

## Restish-Specific Watchlist

These are high-value repo-specific checks, not an exhaustive checklist. Use them when relevant; do not force them into unrelated reviews.

### Subprocess lifecycle

Every `exec.Cmd` that has been `Start()`ed must be `Wait()`ed, or the process becomes a zombie. Error paths usually need cleanup that closes pipes, kills the process if needed, and then waits for exit.

### Timeouts around blocking pipe reads

If a goroutine reads from a subprocess pipe and another goroutine times out, the reader can leak forever unless the read is made interruptible. Closing the relevant `io.ReadCloser` and draining result channels is the usual fix.

### Blocking reads from stdin or other `io.Reader`s

A goroutine blocked on `Read` often cannot be stopped by closing some other resource. Prefer patterns where the blocking read is isolated and the coordinating goroutine can exit cleanly on cancellation.

### Plugin wire compatibility

Backward-incompatible plugin protocol changes usually require updating the plugin API version and checking compatibility behavior for older plugins.

### CBOR byte decoding

CBOR byte payloads may decode into different Go types depending on decoder behavior. Avoid brittle direct assertions when helper functions exist to normalize input.

### Parsed-but-unused config fields

Config fields should not be added unless their behavior is implemented. Otherwise users can set them successfully and still see no effect.

### Generated commands and OpenAPI behavior

Spec-loading or command-generation changes can break existing workflows indirectly. Check for backward compatibility in generated command shape, naming, argument expectations, and operation discovery.

### Output formatting and golden tests

Intentional formatter changes should usually come with targeted regression coverage or golden updates. Unintentional output drift is a common source of user-visible regressions.

### Auth, pagination, filtering, and caching flows

Changes in these areas often regress behavior only in realistic end-to-end paths. Review interactions, not just isolated helpers.

### Test buffer races

Tests that share a `bytes.Buffer` across concurrent writers can hide data races, especially when subprocess stderr/stdout is wired into test buffers.

## Verification Hints

- Run targeted package tests when the changed area is clear.
- Update golden files only when behavior intentionally changed.
- Consider `go test -race ./...` when concurrency, subprocess handling, or shared buffers are touched.

## Example Findings

Good examples illustrate the shape of a finding without hard-coding one exact response style:

- "This changes plugin message semantics but does not bump the plugin API version, so older plugins may silently misbehave."
- "The new timeout returns early, but the goroutine reading from the pipe still blocks forever if the subprocess never responds."
- "The happy-path test passes, but there is no coverage for malformed config or partial server responses."
- "This adds a user-facing flag, but the docs site does not appear to mention it."
