---
name: rsh-simplify
description: Analyze the Restish codebase for simplification and refactoring opportunities, then implement approved changes
---

# Simplifier

Identify and eliminate unnecessary complexity in the Restish codebase. Fewer lines, fewer files, fewer abstraction layers — as long as required features are intact and the user experience is unchanged or better.

## Use This Skill For

- Finding dead code, over-engineered abstractions, or redundant layers in the codebase
- Reorganizing code to be easier to read and maintain
- Collapsing unnecessary indirection: extra packages, tiny single-use interfaces, wrapper types that add no value
- Consolidating scattered logic that belongs together
- Removing options, flags, or config fields that have no real effect
- Replacing hand-rolled logic with standard library equivalents

## Do Not Use This Skill For

- Removing features users rely on
- Weakening tests (tests may be reorganized but coverage must not drop)
- Style-only changes (naming, formatting) with no structural benefit
- Speculative redesigns of areas that are not yet problems

## Priorities

User experience comes first. Developer experience comes second. Simplifications that improve both are highest value. Simplifications that improve only developer experience at some user cost are not acceptable.

## Post-Review Shrink Pass

Follow the "PR Review and Cleanup Discipline" section in AGENTS.md. When shrinking a PR after review loops, target duplicated test setup, repeated assertions, over-large inline fixtures, and helper code that obscures the behavior under test. Prefer deleting ceremony over deleting coverage, and avoid code golf: a shorter diff that is harder to audit, especially around security/auth/cache behavior, is not a simplification.

## Process

### 1. Analyze

Unless the user has pointed at a specific file, package, or area, survey the whole codebase:

```bash
find . -name '*.go' | grep -v '_test.go' | grep -v 'vendor/' | sort
```

Read the key files in `internal/cli/`, the other `internal/` packages, `plugin/`, and `cmd/`. Look specifically for:

- **Dead code**: unexported symbols with no callers, exported symbols not used outside the package, `TODO: remove` comments, disabled or no-op code paths
- **Redundant abstraction**: interfaces with a single implementation that is never swapped out, wrapper types that add no logic, packages that contain a single file with a handful of functions that could live one level up
- **Duplicated logic**: two or more places doing the same thing with no obvious reason to differ
- **Unnecessary indirection**: a function whose entire body is `return otherFunc(args)` with no transformation, extra layers added "for future extensibility" that have never been extended
- **Over-parameterized structs or configs**: fields that are always set to the same value or are never read after being set
- **Test complexity inflating production complexity**: test-only abstractions that forced a production interface into existence

Use grep and structural reading — don't just skim. Pattern:

```bash
# Find unexported functions that might be dead
grep -rn 'func [a-z]' internal/ --include='*.go' | grep -v '_test.go'
# Find single-method interfaces
grep -A2 'type.*interface' internal/ -rn --include='*.go'
```

### 2. Report

Before making any changes, produce a prioritized findings list. Group by category. For each finding:

- Identify the file and line range
- Explain what is unnecessarily complex and why
- Estimate the simplification gain: how many lines deleted, how many files merged, how many abstraction layers removed
- Note any risk: could this break a plugin boundary, user-visible behavior, or test coverage?

Use this structure:

```
## Simplification Opportunities

### High Value
[findings that delete the most code or remove entire files/packages]

### Medium Value
[findings that flatten structure or remove indirection without deleting much]

### Low Value / Polish
[small consolidations or minor clarity improvements]

---
Estimated total: ~N lines removed, M files merged/deleted
```

End the report by asking the user which items to proceed with, or whether to proceed with all of them.

### 3. Implement

Only proceed after the user approves (all items, a subset, or a modified scope). Then:

1. Make the changes, one logical group at a time.
2. After each group, run tests:
   ```bash
   go test ./...
   ```
3. If tests fail, fix the breakage before moving on. Do not skip or delete failing tests — investigate and fix the underlying issue.
4. Build the CLI to confirm compilation:
   ```bash
   go build ./cmd/restish
   ```
5. Build plugins too if plugin-adjacent code changed:
   ```bash
   go build ./cmd/restish-bulk ./cmd/restish-csv ./cmd/restish-mcp
   ```

### 4. Summarize

After all changes, produce a brief summary:

- What was removed or collapsed
- Line/file counts before and after (approximate is fine)
- Any follow-on simplifications that became visible only after the first pass

## Go-Specific Guidance

These patterns are common sources of unnecessary complexity in Go codebases. Treat each as a strong simplification signal:

### Interface over-use
An interface with one implementation that is never injected, mocked in tests, or documented as an extension point is almost always removable. Replace with the concrete type directly. Exception: the `CLI` struct boundary in `internal/cli/` where plugin and test substitution is real.

### Single-file packages
A package that contains one `.go` file (excluding tests) and fewer than ~150 lines is usually a candidate for inlining into its parent. Exception: packages that are imported by external callers (the public `plugin/` API).

### Tiny helper wrappers
Functions of the form:
```go
func wrapThing(x X) Y {
    return someOtherPackage.Thing(x)
}
```
with no added behavior can be deleted; callers can call the wrapped function directly.

### `errors.New` / sentinel errors that are never checked
If an error value is created but no caller ever does `errors.Is(err, ErrFoo)`, it is just a string. Replace with `fmt.Errorf` at the call site and delete the sentinel.

### Config/option structs with always-zero fields
If a struct field is always its zero value in every instantiation across the codebase, the field is dead. Remove it and simplify the struct.

### Exported symbols only used internally
If an exported function or type is only called within its own package, unexport it. This reduces the public API surface and often triggers further simplification.

### Duplicated error-handling boilerplate
Three or more identical sequences of error-check + return scattered through the same file are usually a sign that a small helper or a restructured control flow would be cleaner.

### Nesting depth
If a function has four or more levels of indentation, it is usually doing too much. Extract sub-steps, return early, or flatten with `switch`. Prefer early-return (guard clauses) over deeply nested `if/else` trees.

### Test-only interfaces in production code
If the only reason a production interface exists is to allow mocking in tests, prefer `httptest.Server`, fake implementations, or table-driven integration tests that use the real type.

## Restish-Specific Watchlist

### `internal/cli/` package
This is the largest package. Before merging files here, check whether the split reflects distinct responsibilities or is just historical. Files like `http.go`, `api.go`, `generated.go`, and `request_exec.go` may have overlapping concerns worth consolidating.

### `plugin/` vs `internal/plugin/`
The top-level `plugin/` package is the public contract for external plugin authors; `internal/plugin/` is the host-side implementation. Trace what each package actually exports and whether any wrapping between them is load-bearing before collapsing it.

### `docs/design/` alignment
When removing or collapsing a subsystem, check whether a design doc describes it. If so, update or retire the design doc. Stale design docs are a form of complexity.

## Constraints

- **Never remove user-visible features.** If unsure whether something is user-visible, treat it as if it is.
- **Never weaken test coverage.** Tests may be reorganized, consolidated, or rewritten, but the behaviors they cover must remain covered. Deleting tests requires explicit user approval.
- **Never break the plugin protocol boundary.** The wire format between `restish` and plugin subprocesses is a public contract. Internal reshuffling that does not change the wire format is fine.
- **Never break exported API surface** in the `plugin/` or `cmd/` packages without confirming with the user that a breaking change is acceptable.
- **Prefer building on the standard library** over keeping a thin wrapper around it.
